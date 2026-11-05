#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Build an isolated Plasma/X11 research CLI/helper; never edits the working tree or a VM."""
import argparse
import os
from pathlib import Path
import shutil
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True, help='new absolute directory; binaries are experimental')
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[2]
    output = args.output
    if not output.is_absolute() or output.exists() or any(p.is_symlink() for p in [output, *output.parents]):
        parser.error('output must be new, absolute and have no symlink ancestors')
    for folder in ('cmd', 'internal'):
        if any(p.is_symlink() for p in (repo / folder).rglob('*')):
            parser.error('linked source inputs are not accepted')
    output.mkdir(mode=0o700)
    source = output / 'source'
    source.mkdir()
    for folder in ('cmd', 'internal'):
        shutil.copytree(repo / folder, source / folder)
    for file in ('go.mod', 'go.sum', 'LICENSE'):
        shutil.copyfile(repo / file, source / file)
    catalog = source / 'internal/software/catalog.go'
    data = catalog.read_text().replace('workstation-5.5.0', 'research-plasma-5.5.0')
    for name in ('xfce4-session', 'xfwm4', 'xfdesktop4', 'xfce4-panel', 'xfce4-settings',
                 'xfce4-appfinder', 'thunar', 'xfce4-terminal', 'mousepad', 'greybird-gtk-theme'):
        data = data.replace('"' + name + '", ', '')
    data = data.replace('var DefaultPackages = []string{', 'var DefaultPackages = []string{\n' +
                        ', '.join('"' + p + '"' for p in ('plasma-desktop', 'plasma-workspace', 'kwin-x11',
                        'dolphin', 'konsole', 'kwrite', 'systemsettings', 'breeze-gtk-theme',
                        'qml6-module-qtquick-shapes', 'libgl1-mesa-dri', 'pipewire')) + ',')
    catalog.write_text(data)
    desktop = source / 'internal/desktop/desktop.go'
    data = desktop.read_text().replace('[]string{"xfce4-panel", "xfdesktop"}', '[]string{"plasmashell", "kwin_x11"}')
    replacements = {
        '"/usr/bin/mousepad"': '"/usr/bin/kwrite"', '"Mousepad"': '"kwrite"',
        '"/usr/bin/xfce4-terminal", "--disable-server"': '"/usr/bin/konsole", "--separate"',
        '"Xfce4-terminal"': '"konsole"', '"/usr/bin/thunar", "/home/agent"': '"/usr/bin/dolphin", "/home/agent"',
        '"Thunar"': '"dolphin"', '"/usr/bin/xfce4-settings-manager"': '"/usr/bin/systemsettings"',
        '"Xfce4-settings-manager"': '"systemsettings"',
        '"/usr/bin/xfce4-appfinder", "--disable-server"': '"/usr/bin/krunner"', '"Xfce4-appfinder"': '"krunner"',
    }
    for old, new in replacements.items():
        if old not in data:
            raise RuntimeError('source changed; review experimental adapter: ' + old)
        data = data.replace(old, new)
    desktop.write_text(data)
    template = source / 'internal/lima/template.go'
    data = template.read_text().replace('ReadWritePaths=/home/agent /tmp /run/agent-linux\n',
        'ReadWritePaths=/home/agent /tmp /run/agent-linux /run/agent-linux-desktop\n')
    template.write_text(data)
    launcher = source / 'internal/lima/assets/desktop.sh'
    data = launcher.read_text().replace('if [ -x /usr/local/libexec/agent-linux-workstation ]; then\n /usr/local/libexec/agent-linux-workstation\nfi\n', '')
    data = data.replace('xfconf-query -c xfwm4 -p /general/use_compositing -n -t bool -s false\nxfce4-session &',
        'export KDEWM=/usr/bin/kwin_x11 KWIN_COMPOSE=N QT_QUICK_BACKEND=software LIBGL_ALWAYS_SOFTWARE=true\n'
        'export XDG_SESSION_TYPE=x11 XDG_CURRENT_DESKTOP=KDE DESKTOP_SESSION=plasma\n'
        'kwriteconfig6 --file startkderc --group General --key systemdBoot false\nstartplasma-x11 &')
    data = data.replace('xfce4-panel xfdesktop', 'plasmashell kwin_x11').replace('[ "$i" -lt 100 ]', '[ "$i" -lt 600 ]')
    launcher.write_text(data)
    env = os.environ.copy()
    env.update(GOCACHE=str(repo / '.cache/go-build'), GOMODCACHE=str(repo / '.cache/go-mod'))
    subprocess.run(['go', 'build', '-trimpath', '-o', str(output / 'agent-linux'), './cmd/agent-linux'], cwd=source, env=env, check=True)
    env.update(CGO_ENABLED='0', GOOS='linux', GOARCH='arm64')
    subprocess.run(['go', 'build', '-trimpath', '-ldflags=-s -w', '-o', str(output / 'agent-linux-guest'), './cmd/agent-linux-guest'], cwd=source, env=env, check=True)
    print('Research-only binaries:', output)
    print('No VM created. Use only a new explicitly selected disposable state directory.')


if __name__ == '__main__':
    main()
