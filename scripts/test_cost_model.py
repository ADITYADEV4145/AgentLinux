# SPDX-License-Identifier: Apache-2.0
import copy
import json
from pathlib import Path
import unittest
from cost_model import calculate

class CostModelTests(unittest.TestCase):
    def setUp(self):
        self.config=json.loads((Path(__file__).resolve().parents[1]/'examples/cost-model.json').read_text())
    def test_margin_equation_includes_fees(self):
        for s,r in zip(self.config['scenarios'],calculate(self.config)):
            revenue=r['estimated_required_revenue_per_hour_for_target_margin']
            cost=r['cost_before_payment_per_hour']+r['estimated_payment_per_hour_at_required_revenue']
            self.assertAlmostEqual((revenue-cost)/revenue,s['target_gross_margin'])
    def test_egress_and_waste_increase_required_revenue(self):
        original=calculate(self.config)[0]
        changed=copy.deepcopy(self.config);changed['scenarios'][0]['stream_megabits_per_second']*=2
        self.assertGreater(calculate(changed)[0]['egress_per_hour'],original['egress_per_hour'])
    def test_impossible_margin_rejected(self):
        self.config['scenarios'][0]['target_gross_margin']=1
        with self.assertRaises(ValueError): calculate(self.config)

class CandidatePriceTests(unittest.TestCase):
    def test_low_cost_profit_is_not_a_general_profit_guarantee(self):
        from cost_model import evaluate_prices
        root = Path(__file__).resolve().parents[1]
        config = json.loads((root/'examples/cost-model.json').read_text())
        catalog = json.loads((root/'internal/console/assets/pricing.json').read_text())
        rows = evaluate_prices(config, catalog)
        desktop = [r for r in rows if r['plan'] == 'desktop']
        self.assertTrue(desktop[0]['profitable_under_these_assumptions'])
        self.assertFalse(desktop[2]['profitable_under_these_assumptions'])
        self.assertTrue(all(not r['production_activation_allowed'] for r in rows))
        catalog['plans'][0]['hourly'] = float('nan')
        with self.assertRaises(ValueError): evaluate_prices(config, catalog)

    def test_subscription_full_usage_effective_prices(self):
        from cost_model import evaluate_prices
        root = Path(__file__).resolve().parents[1]
        rows = evaluate_prices(json.loads((root/'examples/cost-model.json').read_text()), json.loads((root/'internal/console/assets/pricing.json').read_text()))
        annual = next(r for r in rows if r['plan'] == 'desktop' and r['billingOption'] == 'annual-full-allowance')
        self.assertAlmostEqual(annual['candidate_price_per_hour'], 200 / 360)
        annual_base = next(r for r in rows if r['plan'] == 'desktop' and r['billingOption'] == 'annual-full-allowance' and r['scenario'] == 'base-assumptions-not-measurements')
        self.assertFalse(annual_base['profitable_under_these_assumptions'])
