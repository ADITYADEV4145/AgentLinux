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
