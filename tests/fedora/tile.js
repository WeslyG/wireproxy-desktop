import Gio from 'gi://Gio';
import GLib from 'gi://GLib';

export function exerciseTile(extension) {
    const dir = GLib.getenv('WIREPROXY_QA_DIR');
    let last = '';
    let command = 0;
    GLib.timeout_add(GLib.PRIORITY_DEFAULT, 100, () => {
        const tile = extension._toggle;
        if (!tile)
            return GLib.SOURCE_REMOVE;
        const state = `${tile.title} | ${tile.subtitle}`;
        if (state !== last && tile.get_parent()) {
            log('WIREPROXY_TILE ' + state);
            last = state;
        }
        const next = Gio.File.new_for_path(`${dir}/click-${command + 1}`);
        if (tile.reactive && !extension._busy && next.query_exists(null)) {
            command++;
            tile.emit('clicked', 1);
            log('WIREPROXY_CLICK ' + command);
        }
        return GLib.SOURCE_CONTINUE;
    });
}
