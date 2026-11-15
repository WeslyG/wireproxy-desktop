# GNOME Quick Settings

The extension targets GNOME Shell 51. The Linux archive installs it with
`sh install.sh --gnome`. After an extension upgrade, sign out of GNOME
and back in to load the new JavaScript.

The app exports `io.github.WeslyG.WireproxyDesktop.QuickSettings` on the
user bus at `/io/github/WeslyG/WireproxyDesktop`.

| D-Bus member | Arguments | Result |
| --- | --- | --- |
| `GetStatus` | — | State and primary profile name, both strings |
| `SetEnabled` | Boolean | Acknowledges queueing a command for the primary profile |
| `OpenWindow` | — | Requests the app window |
| `StatusChanged` | State and primary profile name, both strings | Signal emitted when either changes |

The extension subscribes to status and owner changes before its initial
read. If the idle app has exited, the next click activates it through
D-Bus. The tile keeps the last known primary name while the service is
idle. The `connected` state requires a confirmed handshake for the
primary profile. While another profile is active, the tile does not
offer start or stop.

The older `GetState` method remains available. When changing the
interface, keep this method and the two-string `GetStatus` reply
compatible with previously installed extensions.
