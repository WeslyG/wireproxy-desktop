import Gio from 'gi://Gio';
import GLib from 'gi://GLib';

import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import {QuickToggle, SystemIndicator} from 'resource:///org/gnome/shell/ui/quickSettings.js';

const APP_ID = 'io.github.WeslyG.WireproxyDesktop';
const PATH = '/io/github/WeslyG/WireproxyDesktop';
const INTERFACE = APP_ID + '.QuickSettings';
const START = new Set(['disconnected', 'error']);
const STOP = new Set(['connected', 'connecting', 'unavailable']);
const LABEL = {
    offline: 'Disconnected', loading: 'Loading profiles…',
    'no-profile': 'Choose a primary profile', 'storage-error': 'Profile storage unavailable',
    'other-profile': 'Another profile is active', disconnected: 'Disconnected',
    connecting: 'Connecting…', connected: 'Connected', disconnecting: 'Disconnecting…',
    unavailable: 'Handshake expired', error: 'Connection failed',
};

const RUSSIAN = {
    offline: 'Отключено', loading: 'Загрузка профилей…',
    'no-profile': 'Выберите основной профиль', 'storage-error': 'Хранилище профилей недоступно',
    'other-profile': 'Активен другой профиль', disconnected: 'Отключено',
    connecting: 'Подключение…', connected: 'Подключено', disconnecting: 'Отключение…',
    unavailable: 'Нет ответа от сервера', error: 'Ошибка подключения',
};

export default class WireproxyExtension extends Extension {
    enable() {
        this._russian = GLib.get_language_names()[0].split(/[_\-.]/)[0] === 'ru';
        this._cancel = new Gio.Cancellable();
        this._busy = false;
        this._launchUntil = 0;
        this._launchTimer = 0;
        this._profile = '';
        this._indicator = new SystemIndicator();
        this._toggle = new QuickToggle({title: 'Wireproxy', iconName: 'network-vpn-symbolic'});
        this._toggle.connect('clicked', () => this._click());
        this._indicator.quickSettingsItems.push(this._toggle);
        Main.panel.statusArea.quickSettings.addExternalIndicator(this._indicator);
        this._render('offline');
        this._refreshPending = false;
        this._statusSignal = Gio.DBus.session.signal_subscribe(APP_ID, INTERFACE,
            "StatusChanged", PATH, null, Gio.DBusSignalFlags.NONE, () => this._refresh());
        this._ownerSignal = Gio.DBus.session.signal_subscribe("org.freedesktop.DBus",
            "org.freedesktop.DBus", "NameOwnerChanged", "/org/freedesktop/DBus", APP_ID,
            Gio.DBusSignalFlags.NONE, () => this._refresh());
        this._refresh(true);
    }

    disable() {
        Gio.DBus.session.signal_unsubscribe(this._ownerSignal);
        Gio.DBus.session.signal_unsubscribe(this._statusSignal);
        this._cancel.cancel();
        if (this._launchTimer)
            GLib.Source.remove(this._launchTimer);
        this._toggle.destroy();
        this._indicator.destroy();
    }

    _render(state) {
        this._state = state;
        this._toggle.title = this._profile || 'Wireproxy';
        this._toggle.checked = state === 'connected';
        this._toggle.subtitle = this._launchUntil
            ? (this._russian ? 'Запуск Wireproxy…' : 'Starting Wireproxy…')
            : (this._russian ? RUSSIAN : LABEL)[state];
        this._toggle.reactive = !this._launchUntil &&
            (state === 'offline' || START.has(state) || STOP.has(state));
        this._toggle.can_focus = this._toggle.reactive;
    }

    _call(method, args, callback, activate = false) {
        const cancel = this._cancel;
        const flags = activate ? Gio.DBusCallFlags.NONE : Gio.DBusCallFlags.NO_AUTO_START;
        Gio.DBus.session.call(APP_ID, PATH, `${APP_ID}.QuickSettings`, method, args, null,
            flags, activate ? 10000 : 2000, cancel, (bus, result) => {
                let reply;
                try {
                    reply = bus.call_finish(result);
                } catch {
                    reply = null;
                }
                if (!cancel.is_cancelled())
                    callback(reply);
            });
    }

    _refresh(activate = false) {
        if (this._busy) {
            this._refreshPending = true;
            return;
        }
        this._refreshPending = false;
        this._busy = true;
        this._call('GetStatus', null, reply => {
            this._busy = false;
            const [state, profile] = reply?.deepUnpack() ?? ['offline', this._profile];
            if (state !== 'loading')
                this._profile = profile;
            const launched = this._launchUntil > GLib.get_monotonic_time();
            const start = launched && START.has(state);
            if (!launched || (state !== 'offline' && state !== 'loading'))
                this._launchUntil = 0;
            this._render(Object.hasOwn(LABEL, state) ? state : 'error');
            if (start)
                this._switch(true);
            else if (this._refreshPending)
                this._refresh();
        }, activate);
    }

    _click() {
        if (this._busy || this._launchUntil)
            return;
        if (this._state === 'offline') {
            this._launchUntil = GLib.get_monotonic_time() + 15 * GLib.USEC_PER_SEC;
            this._render('offline');
            if (this._launchTimer)
                GLib.Source.remove(this._launchTimer);
            this._launchTimer = GLib.timeout_add_seconds(GLib.PRIORITY_DEFAULT, 16, () => {
                this._launchTimer = 0;
                if (this._launchUntil) {
                    this._launchUntil = 0;
                    this._render(this._state);
                }
                return GLib.SOURCE_REMOVE;
            });
            this._busy = true;
            this._call('GetStatus', null, reply => {
                this._busy = false;
                if (reply) {
                    this._refresh();
                } else {
                    this._launchUntil = 0;
                    this._render('offline');
                    this._toggle.subtitle = this._russian ? 'Не удалось запустить Wireproxy' : 'Could not start Wireproxy';
                }
            }, true);
        } else if (START.has(this._state) || STOP.has(this._state)) {
            this._switch(START.has(this._state));
        }
    }

    _switch(enabled) {
        this._busy = true;
        this._call('SetEnabled', new GLib.Variant('(b)', [enabled]), () => {
            this._busy = false;
            this._refresh();
        });
    }
}
