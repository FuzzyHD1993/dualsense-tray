package main

// dualsense-tray — иконка DualSense в области статуса COSMIC (SNI).
// Клик переключает профили (dualsense-toggle), иконка показывает состояние.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	itemPath    = "/StatusNotifierItem"
	itemIface   = "org.kde.StatusNotifierItem"
	propIface   = "org.freedesktop.DBus.Properties"
	watcher     = "org.kde.StatusNotifierWatcher"
	watcherPath = "/StatusNotifierWatcher"
)

type pixmap struct {
	W, H int32
	Data []byte
}

type tooltip struct {
	IconName string
	Pixmaps  []pixmap
	Title    string
	Subtitle string
}

type sni struct {
	conn     *dbus.Conn
	busName  string
	disabled bool
}

func home() string { h, _ := os.UserHomeDir(); return h }

func offPath() string { return filepath.Join(home(), ".config/dualsense/disabled") }

func isDisabled() bool {
	_, err := os.Stat(offPath())
	return err == nil
}

func (s *sni) iconName() string {
	if s.disabled {
		return "dualsense-off-symbolic"
	}
	return "dualsense-on-symbolic"
}

func (s *sni) title() string {
	if s.disabled {
		return "DualSense: профили выключены"
	}
	return "DualSense: профили включены"
}

// --- методы SNI ---

func (s *sni) Activate(x, y int32) *dbus.Error                    { s.toggle(); return nil }
func (s *sni) SecondaryActivate(x, y int32) *dbus.Error           { s.toggle(); return nil }
func (s *sni) ContextMenu(x, y int32) *dbus.Error                 { s.toggle(); return nil }
func (s *sni) Scroll(delta int32, orientation string) *dbus.Error { return nil }

func (s *sni) toggle() {
	bin := filepath.Join(home(), ".local/bin/dualsense-toggle")
	_ = exec.Command(bin).Run()
	s.refresh()
}

func (s *sni) refresh() {
	was := s.disabled
	s.disabled = isDisabled()
	if was == s.disabled {
		return
	}
	_ = s.conn.Emit(itemPath, itemIface+".NewIcon")
	_ = s.conn.Emit(itemPath, itemIface+".NewToolTip")
	_ = s.conn.Emit(itemPath, itemIface+".NewTitle")
	_ = s.conn.Emit(itemPath, propIface+".PropertiesChanged", itemIface,
		map[string]dbus.Variant{
			"IconName": dbus.MakeVariant(s.iconName()),
			"Title":    dbus.MakeVariant(s.title()),
		}, []string{})
}

// --- свойства DBus ---

func (s *sni) Get(iface string, prop string) (dbus.Variant, *dbus.Error) {
	switch prop {
	case "Category":
		return dbus.MakeVariant("ApplicationStatus"), nil
	case "Id":
		return dbus.MakeVariant("dualsense-tray"), nil
	case "Title":
		return dbus.MakeVariant(s.title()), nil
	case "Status":
		return dbus.MakeVariant("Active"), nil
	case "IconName":
		return dbus.MakeVariant(s.iconName()), nil
	case "IconThemePath":
		return dbus.MakeVariant(""), nil
	case "ItemIsMenu":
		return dbus.MakeVariant(false), nil
	case "Menu":
		return dbus.MakeVariant(dbus.ObjectPath("/NO_DBUSMENU")), nil
	case "ToolTip":
		return dbus.MakeVariant(tooltip{
			IconName: s.iconName(),
			Pixmaps:  []pixmap{},
			Title:    s.title(),
			Subtitle: "Клик — включить/выключить",
		}), nil
	}
	return dbus.MakeVariant(""), nil
}

func (s *sni) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	props := []string{"Category", "Id", "Title", "Status", "IconName", "ItemIsMenu", "Menu", "ToolTip", "IconThemePath"}
	m := make(map[string]dbus.Variant, len(props))
	for _, p := range props {
		v, _ := s.Get(iface, p)
		m[p] = v
	}
	return m, nil
}

func (s *sni) register() error {
	obj := s.conn.Object(watcher, watcherPath)
	call := obj.Call(watcher+".RegisterStatusNotifierItem", 0, s.busName)
	return call.Err
}

func main() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "session bus:", err)
		os.Exit(1)
	}
	defer conn.Close()

	name := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())
	reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		fmt.Fprintln(os.Stderr, "request name:", err, reply)
		os.Exit(1)
	}

	s := &sni{conn: conn, busName: name, disabled: isDisabled()}
	conn.Export(s, itemPath, itemIface)
	conn.Export(s, itemPath, propIface)

	if err := s.register(); err != nil {
		fmt.Fprintln(os.Stderr, "register:", err)
	}

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	rereg := time.NewTicker(30 * time.Second)
	defer rereg.Stop()

	for {
		select {
		case <-tick.C:
			s.refresh()
		case <-rereg.C:
			if !watcherHas(conn, name) {
				_ = s.register()
			}
		}
	}
}

func watcherHas(conn *dbus.Conn, name string) bool {
	obj := conn.Object(watcher, watcherPath)
	v, err := obj.GetProperty(watcher + ".RegisteredStatusNotifierItems")
	if err != nil {
		return false
	}
	items, ok := v.Value().([]string)
	if !ok {
		return false
	}
	for _, it := range items {
		if it == name {
			return true
		}
	}
	return false
}
