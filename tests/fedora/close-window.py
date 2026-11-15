# Send the same WM_DELETE_WINDOW message as an X11 window manager.
import ctypes as c
import sys

x11 = c.CDLL("libX11.so.6")
x11.XOpenDisplay.argtypes = [c.c_char_p]
x11.XOpenDisplay.restype = c.c_void_p
x11.XInternAtom.argtypes = [c.c_void_p, c.c_char_p, c.c_int]
x11.XInternAtom.restype = c.c_ulong

class Message(c.Structure):
    _fields_ = [
        ("type", c.c_int), ("serial", c.c_ulong), ("send_event", c.c_int),
        ("display", c.c_void_p), ("window", c.c_ulong), ("message_type", c.c_ulong),
        ("format", c.c_int), ("data", c.c_long * 5),
    ]

display = x11.XOpenDisplay(None)
if not display:
    raise SystemExit("X11 display is unavailable")
window = int(sys.argv[1])
protocols = x11.XInternAtom(display, b"WM_PROTOCOLS", 0)
delete = x11.XInternAtom(display, b"WM_DELETE_WINDOW", 0)
event = (c.c_long * 24)()
c.cast(event, c.POINTER(Message))[0] = Message(
    33, 0, 1, display, window, protocols, 32, (c.c_long * 5)(delete)
)
x11.XSendEvent.argtypes = [c.c_void_p, c.c_ulong, c.c_int, c.c_long, c.c_void_p]
x11.XCloseDisplay.argtypes = [c.c_void_p]
if not x11.XSendEvent(display, window, 0, 0, c.byref(event)):
    raise SystemExit("Could not send WM_DELETE_WINDOW")
x11.XCloseDisplay(display)
