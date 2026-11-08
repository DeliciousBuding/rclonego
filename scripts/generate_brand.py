"""Generate brand SVGs from the monochrome mark without changing its geometry."""
import argparse
from pathlib import Path
import xml.etree.ElementTree as ET

root = Path(__file__).resolve().parents[1]
source = root / "assets/mark-mono.svg"
namespace = "{http://www.w3.org/2000/svg}"
symbol = ET.parse(source).getroot().find(namespace + "g")
if symbol is None:
    raise SystemExit("The monochrome mark has no symbol group.")

attributes = dict(symbol.attrib)
paths = "".join(
    f'<path d="{path.attrib["d"]}"/>'
    for path in symbol.findall(namespace + "path")
)
flow = (
    '<defs><linearGradient id="flow" gradientUnits="userSpaceOnUse" '
    'x1="11" y1="15" x2="53" y2="50">'
    '<stop stop-color="#9adde5"/>'
    '<stop offset="1" stop-color="#72b9de"/>'
    '</linearGradient></defs>'
)


def mark(stroke):
    return (
        f'<g fill="{attributes["fill"]}" stroke="{stroke}" '
        f'stroke-width="{attributes["stroke-width"]}" '
        f'stroke-linecap="{attributes["stroke-linecap"]}" '
        f'stroke-linejoin="{attributes["stroke-linejoin"]}">'
        f'{paths}</g>'
    )


generated = {
    "assets/mark.svg": (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" '
        'width="64" height="64">'
        f'{flow}{mark("url(#flow)")}</svg>\n'
    ),
    "assets/icon.svg": (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" '
        'width="256" height="256">'
        '<defs><linearGradient id="tile" x1="0" y1="0" x2=".8" y2="1">'
        '<stop stop-color="#ffffff"/><stop offset="1" stop-color="#f1f6f8"/>'
        '</linearGradient></defs>'
        '<rect x="9" y="9" width="238" height="238" rx="53" '
        'fill="url(#tile)" stroke="#e4ecef" stroke-width="1.1"/>'
        '<g transform="translate(44.8 43.8) scale(2.6)">'
        f'{flow}{mark("url(#flow)")}</g></svg>\n'
    ),
    "assets/wordmark.svg": (
        '<svg xmlns="http://www.w3.org/2000/svg" width="480" height="104" '
        'viewBox="0 0 480 104">\n'
        '  <g transform="scale(1.5)">'
        f'{mark("#318b9d")}</g>\n'
        '  <text x="104" y="66" fill="#318b9d" '
        'font-family="system-ui,-apple-system,Segoe UI,sans-serif" font-size="46" '
        'font-weight="600" letter-spacing="-1.4">RcloneGo</text>\n'
        '</svg>\n'
    ),
    "internal/ui/assets/app-mark.svg": (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" '
        'width="64" height="64">'
        f'{mark("currentColor")}</svg>\n'
    ),
}

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--check", action="store_true", help="check generated files without writing")
args = parser.parse_args()
for name, content in generated.items():
    file = root / name
    if args.check:
        if not file.is_file() or file.read_text(encoding="utf-8") != content:
            raise SystemExit(f"{file.relative_to(root)} differs; run scripts/generate_brand.py")
    else:
        file.write_text(content, encoding="utf-8", newline="\n")
print("Brand SVGs match the source mark." if args.check else "Generated mark, icon, wordmark and runtime SVGs.")
