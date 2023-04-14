# sway-fader

sway-fader fades in windows on workspace switch and window creation.

Example config in ~/.config/sway/config to keep `foot` terminal transparent and other windows opaque:

```
for_window [app_id="foot"] opacity 0.97
exec sway-fader --app_id="foot:0.7:0.97"
```

# Install

```
go install github.com/mgnsk/sway-fader@latest
```
