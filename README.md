# sway-fader

sway-fader fades in all windows when a workspace is focused.

Usual config in ~/.config/sway/config to keep terminals transparent and other windows opaque:

```
for_window [app_id="foot"] opacity 0.97
exec sway-fader --app_id="foot:0.7:0.97"
```
