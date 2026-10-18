import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from open_mind_tui.app import OpenMindTUI


class ModelDiscoveryTests(unittest.TestCase):
    def test_lists_only_top_level_omf_models_in_name_order(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            models = root / "models"
            models.mkdir()
            (models / "zeta.omf").write_bytes(b"")
            (models / "Alpha.omf").write_bytes(b"")
            (models / "ignore.gguf").write_bytes(b"")
            (models / "folder.omf").mkdir()

            with patch.dict(os.environ, {"OPEN_MIND_HOME": temporary}):
                app = OpenMindTUI(runtime=Path("/missing/om"), source=None, host="127.0.0.1", port=8080)
                self.assertEqual([item.name for item in app.models()], ["Alpha.omf", "zeta.omf"])

    def test_missing_model_directory_is_an_empty_library(self):
        with tempfile.TemporaryDirectory() as temporary:
            with patch.dict(os.environ, {"OPEN_MIND_HOME": temporary}):
                app = OpenMindTUI(runtime=Path("/missing/om"), source=None, host="127.0.0.1", port=8080)
                self.assertEqual(app.models(), [])


if __name__ == "__main__":
    unittest.main()
