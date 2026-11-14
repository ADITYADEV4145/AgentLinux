# SPDX-License-Identifier: Apache-2.0
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import build_site


class PublicSiteTests(unittest.TestCase):
    def test_public_allowlist_and_routes(self):
        with tempfile.TemporaryDirectory() as root:
            output=Path(root)/'public'
            manifest=build_site.build(output)
            self.assertEqual(manifest,json.loads((output/'.build-manifest.json').read_text()))
            self.assertTrue((output/'brand/agent-linux-logo.png').is_file())
            self.assertTrue((output/'fonts/OFL.txt').is_file())
            self.assertTrue((output/'get-started/index.html').is_file())
            self.assertFalse((output/'dashboard').exists())
            self.assertFalse((output/'app.js').exists())
            self.assertFalse((output/'api').exists())
            text=(output/'index.html').read_text()
            self.assertIn('/public-bridge.js',text)
            self.assertNotIn('/landing-bridge.js',text)
            bridge=(output/'public-bridge.js').read_text()
            self.assertNotIn('127.0.0.1',bridge)
            self.assertNotIn('fetch(',bridge)
            self.assertIn('/get-started/',bridge)
            self.assertTrue(all(not p.endswith(('.go','.pem','.key','.qcow2')) for p in manifest))

    def test_repeatable_build_and_modified_output_refused(self):
        with tempfile.TemporaryDirectory() as root:
            output=Path(root)/'public'
            with patch.object(build_site,'OUTPUT',output):
                build_site.main()
                first=(output/'.build-manifest.json').read_bytes()
                build_site.main()
                self.assertEqual(first,(output/'.build-manifest.json').read_bytes())
                (output/'private.txt').write_text('must not be removed or uploaded')
                with self.assertRaisesRegex(ValueError,'unexpected files'):
                    build_site.main()
                self.assertTrue((output/'private.txt').exists())

if __name__=='__main__':unittest.main()
