# SPDX-License-Identifier: Apache-2.0
"""Installer boundary/failure tests; dummy bytes are never executed."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('installer', Path(__file__).with_name('install.py'))
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name).resolve()
        self.prefix = self.root / 'tools'
        self.old_umask = os.umask(0o077)

    def tearDown(self):
        os.umask(self.old_umask)
        self.tmp.cleanup()

    def bundle(self, commit='a' * 40):
        folder = self.root / commit
        folder.mkdir()
        records = {}
        for name in ['bin/agent-linux', 'bin/agent-linux-guest', 'LICENSE', 'README.md', 'install.py']:
            p = folder / name
            p.parent.mkdir(exist_ok=True)
            data = ('fixture-' + name + commit).encode()
            p.write_bytes(data)
            records[name] = {'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}
        (folder / 'manifest.json').write_text(json.dumps({'version': '0.5.0-rc.1', 'commit': commit,
                                                        'target': 'darwin-arm64', 'dirty': False, 'files': records}))
        return folder

    def test_install_upgrade_uninstall_preserves_vm(self):
        first = self.bundle()
        installed = installer.install(first, self.prefix)
        self.assertTrue(installed.is_file())
        old = os.readlink(self.prefix / 'current')
        second = self.bundle('b' * 40)
        installer.install(second, self.prefix)
        self.assertNotEqual(os.readlink(self.prefix / 'current'), old)
        vm = self.root / 'vm-data'
        vm.write_text('unrelated')
        with self.assertRaises(ValueError):
            installer.uninstall(self.prefix, False)
        installer.uninstall(self.prefix, True)
        self.assertEqual(vm.read_text(), 'unrelated')
        self.assertFalse(installed.exists())
        self.assertTrue((self.prefix / '.install-lock').exists())

    def test_tamper_and_partial_upgrade_leave_current(self):
        first = self.bundle()
        installer.install(first, self.prefix)
        old = os.readlink(self.prefix / 'current')
        second = self.bundle('b' * 40)
        (second / 'bin/agent-linux').write_text('corrupt')
        with self.assertRaises(ValueError):
            installer.install(second, self.prefix)
        self.assertEqual(os.readlink(self.prefix / 'current'), old)
        third = self.bundle('c' * 40)
        with patch.object(installer.os, 'replace', side_effect=OSError('activation failed')):
            with self.assertRaises(OSError):
                installer.install(third, self.prefix)
        self.assertEqual(os.readlink(self.prefix / 'current'), old)
        self.assertFalse(any(p.name.startswith('.install-') for p in (self.prefix / 'releases').iterdir()))

    def test_path_links_permissions_and_unknown_data(self):
        bundle = self.bundle()
        (bundle / 'bin/agent-linux').unlink()
        (bundle / 'bin/agent-linux').symlink_to('/etc/passwd')
        with self.assertRaises((ValueError, OSError)):
            installer.install(bundle, self.prefix)
        self.assertFalse(self.prefix.exists())
        bundle2 = self.bundle('b' * 40)
        installer.install(bundle2, self.prefix)
        (self.prefix / 'personal').write_text('safe')
        with self.assertRaises(ValueError):
            installer.uninstall(self.prefix, True)
        self.assertEqual((self.prefix / 'personal').read_text(), 'safe')
        (self.prefix / 'personal').unlink()
        self.prefix.chmod(0o755)
        with self.assertRaises(ValueError):
            installer.uninstall(self.prefix, True)
        self.prefix.chmod(0o700)

    def test_traversal_in_manifest_and_symlink_prefix(self):
        bundle = self.bundle()
        manifest = json.loads((bundle / 'manifest.json').read_text())
        manifest['files']['../escape'] = {'size': 0, 'sha256': '0' * 64}
        (bundle / 'manifest.json').write_text(json.dumps(manifest))
        with self.assertRaises((ValueError, OSError)):
            installer.install(bundle, self.prefix)
        self.prefix.symlink_to(self.root)
        with self.assertRaises(ValueError):
            installer.uninstall(self.prefix, True)


class PackagingInputTests(unittest.TestCase):
    def test_symlinks_and_traversal_cannot_enter_bundle(self):
        package_spec = importlib.util.spec_from_file_location('packager', Path(__file__).with_name('package.py'))
        packager = importlib.util.module_from_spec(package_spec)
        package_spec.loader.exec_module(packager)
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            repo, out = root / 'repo', root / 'out'
            repo.mkdir()
            out.mkdir()
            (root / 'private').write_text('must never be packaged')
            (repo / 'link').symlink_to(root / 'private')
            for name in ['link', '../private', str(root / 'private')]:
                with self.assertRaises(ValueError):
                    packager.copy_reviewed(repo, name, out)
            self.assertEqual(list(out.iterdir()), [])


if __name__ == '__main__':
    unittest.main()
