# sway-fader

sway-fader fades in containers on workspace focus and window new events.

# Configuration

```
exec sway-fader
```

By default a rule of `--class=".*:0.7:1.0"` is applied which can be overridden by user specified flags.

# Advanced config

Example config in ~/.config/sway/config to keep `foot` terminal transparent and other windows opaque:

```
exec sway-fader --app_id="foot:0.7:0.97"
```
