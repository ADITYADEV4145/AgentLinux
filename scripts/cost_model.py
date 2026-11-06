#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Editable assumption-based cost model. No API calls or actual pricing claims."""
import argparse
import json
import math
from pathlib import Path


def calculate(config):
    results = []
    for s in config['scenarios']:
        def n(k):
            v = float(s[k])
            if not math.isfinite(v) or v < 0:
                raise ValueError('nonnegative finite assumption required: ' + k)
            return v
        hours = n('monthly_billable_machine_hours')
        invoice_hours = n('mean_machine_hours_per_invoice')
        margin, processing = n('target_gross_margin'), n('payment_processing_fraction')
        if not hours or not invoice_hours or margin + processing >= 1:
            raise ValueError('positive hours and target margin + processing < 1 required')
        compute = n('cpus') * n('cpu_cost_per_hour')
        memory = n('memory_gib') * n('memory_cost_per_gib_hour')
        storage = n('disk_gib') * n('storage_cost_per_gib_month') / 730 * n('storage_lifetime_multiplier')
        stream_gib = n('stream_megabits_per_second') * 1_000_000 / 8 * 3600 / 1024**3
        egress = (stream_gib + n('other_egress_gib_per_hour')) * n('egress_cost_per_gib')
        provider = n('provider_overhead_per_machine_hour')
        infrastructure = compute + memory + storage + egress + provider
        wasted = infrastructure * (n('trial_fraction') + n('idle_orphan_fraction'))
        control = (n('database_monthly') + n('control_plane_monthly')) / hours
        support = n('support_hours_monthly') * n('support_cost_per_hour') / hours
        fixed_payment = n('payment_fixed_fee_per_invoice') / invoice_hours
        cost = infrastructure + wasted + control + support
        required = (cost + fixed_payment) / (1 - margin - processing)
        results.append({'scenario': s['name'], 'estimate_only': True,
                        'infrastructure_per_hour': infrastructure,
                        'stream_gib_per_hour': stream_gib,
                        'egress_per_hour': egress, 'trial_idle_orphan_per_hour': wasted,
                        'database_control_per_hour': control, 'support_per_hour': support,
                        'cost_before_payment_per_hour': cost,
                        'estimated_required_revenue_per_hour_for_target_margin': required,
                        'estimated_payment_per_hour_at_required_revenue': required * processing + fixed_payment,
                        'target_gross_margin': margin,
                        'retail_price_finalized': False})
    return results


def evaluate_prices(config, catalog):
    """Stress-test targets from the same catalog shown by the local website.

    This assumes linear CPU/RAM/disk infrastructure scaling. Support and control
    assumptions remain unchanged. It is an estimate, never an activation path.
    """
    import copy
    rows = []
    variants = []
    for original in catalog['plans']:
        variants.append({**original, 'billingOption': 'hourly'})
        sub = original.get('subscription')
        if sub:
            hours = float(sub['includedHoursPerMonth'])
            if not math.isfinite(hours) or hours <= 0:
                raise ValueError('positive finite monthly hours required')
            variants.append({**original, 'billingOption': 'monthly-full-allowance', 'hourly': sub['monthlyUSD'] / hours})
            variants.append({**original, 'billingOption': 'annual-full-allowance', 'hourly': sub['annualUSD'] / (hours * 12)})
    for plan in variants:
        price = float(plan['hourly'])
        if not math.isfinite(price) or price <= 0:
            raise ValueError('positive finite candidate price required')
        for original in config['scenarios']:
            scenario = copy.deepcopy(original)
            for source, target in [('cpus', 'cpus'), ('memoryGiB', 'memory_gib'), ('diskGiB', 'disk_gib')]:
                size = float(plan[source])
                if not math.isfinite(size) or size <= 0:
                    raise ValueError('positive finite machine size required')
                scenario[target] = size
            result = calculate({'scenarios': [scenario]})[0]
            payments = price * scenario['payment_processing_fraction'] + scenario['payment_fixed_fee_per_invoice'] / scenario['mean_machine_hours_per_invoice']
            cost = result['cost_before_payment_per_hour'] + payments
            margin = (price - cost) / price
            rows.append({'plan': plan['id'], 'billingOption': plan['billingOption'], 'scenario': scenario['name'],
                         'estimate_only': True, 'candidate_price_per_hour': price,
                         'estimated_cost_including_payments_per_hour': cost,
                         'estimated_contribution_per_hour': price - cost,
                         'estimated_margin_fraction': margin,
                         'profitable_under_these_assumptions': margin > 0,
                         'meets_target_margin': margin >= scenario['target_gross_margin'],
                         'production_activation_allowed': False})
    return rows


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--config', type=Path, default=Path(__file__).resolve().parents[1] / 'examples/cost-model.json')
    p.add_argument('--catalog', type=Path, help='optional proposed website pricing catalog; stress-test each target')
    args = p.parse_args()
    config = json.loads(args.config.read_text())
    result = evaluate_prices(config, json.loads(args.catalog.read_text())) if args.catalog else calculate(config)
    print(json.dumps(result, indent=2, allow_nan=False))


if __name__ == '__main__':
    main()
