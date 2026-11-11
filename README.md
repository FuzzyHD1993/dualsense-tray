# dualsense-tray

A DualSense icon in the **COSMIC panel status area** (StatusNotifierItem).
One click switches between *our* trigger profiles and the game's own effects.

```
┌─────────────────────────────┐
│  🎮  ...  23:41   🔊  ⏻     │   ← click the gamepad
└─────────────────────────────┘
```

- icon reflects the state: gamepad (our profiles on) / crossed-out (off)
- click / secondary click toggles `dualsense-toggle`
- state is read from `~/.config/dualsense/disabled`
- automatically re-registers if the status area restarts

## Requirements

- COSMIC with `cosmic-applet-status-area` in the panel
- [dualsense-autoswitch](https://github.com/FuzzyHD1993/dualsense-autoswitch) for the
  `dualsense-toggle` command

## Install

```bash
git clone https://github.com/FuzzyHD1993/dualsense-tray
cd dualsense-tray
./install.sh
```

## Icons

`icons/dualsense-on-symbolic.svg` and `icons/dualsense-off-symbolic.svg` are
installed into `~/.local/share/icons/hicolor/scalable/apps`.

## License

MIT
