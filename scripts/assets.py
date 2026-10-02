import json
from pathlib import Path

root = Path(__file__).resolve().parent.parent
runtime = (root / "runtime/runtime.m").read_text()
sdk = (root / "ios/ios.go").read_text()

stdlib = {}
for path in sorted((root / "stdlib").rglob("*.go")):
    import_path = path.relative_to(root / "stdlib").parent.as_posix()
    stdlib[import_path] = path.read_text()

output = ["package compiler", "", "const RuntimeSource = " + json.dumps(runtime, ensure_ascii=False), "", "const SDKSource = " + json.dumps(sdk, ensure_ascii=False), "", "var stdlibSources = map[string]string{"]
for import_path, source in sorted(stdlib.items()):
    output.append("\t" + json.dumps(import_path) + ": " + json.dumps(source, ensure_ascii=False) + ",")
output.append("}")
output.append("")
(root / "internal/compiler/assets.go").write_text("\n".join(output))