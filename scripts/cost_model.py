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


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--config', type=Path, default=Path(__file__).resolve().parents[1] / 'examples/cost-model.json')
    args = p.parse_args()
    print(json.dumps(calculate(json.loads(args.config.read_text())), indent=2, allow_nan=False))


if __name__ == '__main__':
    main()
