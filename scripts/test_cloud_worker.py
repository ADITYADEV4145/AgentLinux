# SPDX-License-Identifier: Apache-2.0
"""Offline SDK-double checks; never installs/imports a real provider SDK."""
import ast
import importlib.util
import os
from pathlib import Path
import sys
import types
import unittest
from unittest.mock import patch

PATH = Path(__file__).resolve().parents[1] / 'internal/cloud/e2b_worker.py'
spec = importlib.util.spec_from_file_location('cloud_worker', PATH)
worker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(worker)

class WorkerTests(unittest.TestCase):
    def test_embedded_guest_code_compiles_without_nul(self):
        tree = ast.parse(PATH.read_text())
        found = []
        for node in ast.walk(tree):
            if isinstance(node, ast.Assign) and any(isinstance(x, ast.Name) and x.id == 'guest' for x in node.targets):
                found.append(node.value.value)
        self.assertEqual(len(found), 1)
        self.assertNotIn('\0', found[0])
        compile(found[0], '<fixed guest runner>', 'exec')

    def test_disabled_before_importing_sdk(self):
        with patch.dict(os.environ, {}, clear=True):
            with self.assertRaises(worker.ContractError):
                worker.run({'method': 'create'})

    def test_create_sdk_arguments_and_remaining_deadline(self):
        captured = {}
        class Sandbox:
            @staticmethod
            def create(**kwargs):
                captured.update(kwargs)
                return types.SimpleNamespace(sandbox_id='test-sandbox')
        class NotFound(Exception): pass
        mods = {'PIL': types.SimpleNamespace(Image=object()), 'e2b_desktop': types.SimpleNamespace(Sandbox=Sandbox),
                'e2b': types.SimpleNamespace(Sandbox=Sandbox),
                'e2b.sandbox.sandbox_api': types.SimpleNamespace(SandboxQuery=object, SandboxState=object),
                'e2b.exceptions': types.SimpleNamespace(NotFoundException=NotFound)}
        # PIL is a local image dependency, never a provider or API dependency.
        with patch.dict(sys.modules, mods), patch.dict(os.environ, {'AGENT_LINUX_ENABLE_E2B':'operator-enabled'}), patch.object(worker, 'version', return_value='2.7.2'):
            import datetime
            expiry=(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(seconds=45)).isoformat()
            result=worker.run({'method':'create','deployment':'deployment','session':{'id':'session','expires':expiry},'template':'reviewed'})
        self.assertEqual(result['resource']['id'], 'test-sandbox')
        self.assertEqual(captured['metadata']['agent_linux_session'],'session')
        self.assertEqual(captured['idempotency_key'],'session')
        self.assertGreater(captured['timeout'],0)
        self.assertLessEqual(captured['timeout'],45)
        self.assertEqual(captured['lifecycle'],{'on_timeout':'kill','auto_resume':False})
        self.assertFalse(captured['network']['allow_public_traffic'])
        self.assertIn('169.254.0.0/16', captured['network']['deny_out'])

if __name__=='__main__': unittest.main()
