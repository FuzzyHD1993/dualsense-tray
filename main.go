package main

// dualsense-tray — иконка DualSense в области статуса COSMIC (SNI + DBusMenu).
// Левый клик — тумблер профилей, правый — выбор профиля.
// Состояние берётся из ~/.config/dualsense/{override,disabled}.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	itemPath    = "/StatusNotifierItem"
	itemIface   = "org.kde.StatusNotifierItem"
	menuPath    = "/MenuBar"
	menuIface   = "com.canonical.dbusmenu"
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

type menuLayout struct {
	ID       int32
	Props    map[string]dbus.Variant
	Children []dbus.Variant
}

type sni struct {
	conn    *dbus.Conn
	busName string
}

func home() string { h, _ := os.UserHomeDir(); return h }

func offPath() string        { return filepath.Join(home(), ".config/dualsense/disabled") }
func overridePath() string   { return filepath.Join(home(), ".config/dualsense/override") }
func bin(name string) string { return filepath.Join(home(), ".local/bin", name) }

func isDisabled() bool {
	_, err := os.Stat(offPath())
	return err == nil
}

func override() string {
	b, err := os.ReadFile(overridePath())
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(b))
	if s == "auto" || s == "off" {
		return ""
	}
	return s
}

func autoProfile() string {
	out, err := exec.Command(bin("dualsense-autoswitch"), "-once").Output()
	if err != nil {
		return "default"
	}
	return strings.TrimSpace(string(out))
}

func (s *sni) mode() (kind, name string) {
	if isDisabled() {
		return "off", ""
	}
	if p := override(); p != "" {
		return "pinned", p
	}
	return "auto", autoProfile()
}

func (s *sni) iconName() string {
	if isDisabled() {
		return "dualsense-off-symbolic"
	}
	return "dualsense-on-symbolic"
}

func (s *sni) title() string {
	kind, name := s.mode()
	switch kind {
	case "off":
		return "DualSense: выключено"
	case "pinned":
		return "DualSense: профиль " + name
	default:
		return "DualSense: авто · " + name
	}
}

func (s *sni) refresh() {
	_ = s.conn.Emit(itemPath, itemIface+".NewIcon")
	_ = s.conn.Emit(itemPath, itemIface+".NewToolTip")
	_ = s.conn.Emit(itemPath, itemIface+".NewTitle")
	_ = s.conn.Emit(itemPath, propIface+".PropertiesChanged", itemIface,
		map[string]dbus.Variant{
			"IconName": dbus.MakeVariant(s.iconName()),
			"Title":    dbus.MakeVariant(s.title()),
		}, []string{})
	_ = s.conn.Emit(menuPath, menuIface+".LayoutUpdated", uint32(1), int32(0))
}

// --- SNI ---

func (s *sni) Activate(x, y int32) *dbus.Error                    { s.toggle(); return nil }
func (s *sni) SecondaryActivate(x, y int32) *dbus.Error           { s.toggle(); return nil }
func (s *sni) ContextMenu(x, y int32) *dbus.Error                 { return nil }
func (s *sni) Scroll(delta int32, orientation string) *dbus.Error { return nil }

func (s *sni) toggle() {
	_ = exec.Command(bin("dualsense-toggle")).Run()
	s.refresh()
}

func (s *sni) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
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
	case "ItemIsMenu":
		return dbus.MakeVariant(false), nil
	case "Menu":
		return dbus.MakeVariant(dbus.ObjectPath(menuPath)), nil
	case "ToolTip":
		return dbus.MakeVariant(tooltip{
			IconName: s.iconName(),
			Pixmaps:  []pixmap{},
			Title:    s.title(),
			Subtitle: "ЛКМ — включить/выключить, ПКМ — профиль",
		}), nil
	}
	return dbus.MakeVariant(""), nil
}

func (s *sni) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	m := map[string]dbus.Variant{}
	for _, p := range []string{"Category", "Id", "Title", "Status", "IconName", "ItemIsMenu", "Menu", "ToolTip"} {
		v, _ := s.Get(iface, p)
		m[p] = v
	}
	return m, nil
}

// --- DBusMenu ---

type menuDef struct {
	id      int32
	label   string
	prof    string // профиль для dualsense-profile-set; "" = особый пункт
	special string // "auto" | "off" | "sep"
}

var menuItems = []menuDef{
	{id: 1, label: "Авто (по игре)", special: "auto"},
	{id: 2, label: "Выключено", special: "off"},
	{id: 3, label: "", special: "sep"},
	{id: 10, label: "AW2", prof: "aw2"},
	{id: 11, label: "AW2 · лук", prof: "aw2-bow"},
	{id: 12, label: "AW2 · дробовик", prof: "aw2-shotgun"},
	{id: 13, label: "RDR2", prof: "rdr2"},
	{id: 14, label: "RDR2 · лук", prof: "rdr2-bow"},
	{id: 15, label: "PoE2", prof: "poe2"},
	{id: 16, label: "PoE2 · лук", prof: "poe2-bow"},
	{id: 17, label: "Crimson Desert", prof: "cd"},
	{id: 18, label: "ETS2", prof: "ets2"},
	{id: 19, label: "Cyberpunk 2077", prof: "cp2077"},
}

func (s *sni) activeID() int32 {
	kind, name := s.mode()
	switch kind {
	case "off":
		return 2
	case "pinned":
		for _, it := range menuItems {
			if it.prof == name {
				return it.id
			}
		}
		return 1
	default:
		return 1
	}
}

func (s *sni) properties(id int32) map[string]dbus.Variant {
	props := map[string]dbus.Variant{"visible": dbus.MakeVariant(true)}
	for _, it := range menuItems {
		if it.id != id {
			continue
		}
		switch it.special {
		case "sep":
			props["type"] = dbus.MakeVariant("separator")
			return props
		}
		props["label"] = dbus.MakeVariant(it.label)
		props["enabled"] = dbus.MakeVariant(true)
		if it.special == "auto" || it.special == "off" || it.prof != "" {
			props["toggle-type"] = dbus.MakeVariant("radio")
			state := int32(0)
			if s.activeID() == it.id {
				state = 1
			}
			props["toggle-state"] = dbus.MakeVariant(state)
		}
		return props
	}
	return props
}

func (s *sni) layout(id int32, depth int32) menuLayout {
	l := menuLayout{ID: id, Props: map[string]dbus.Variant{}}
	if id == 0 {
		for _, it := range menuItems {
			l.Children = append(l.Children, dbus.MakeVariant(s.layout(it.id, depth-1)))
		}
		return l
	}
	l.Props = s.properties(id)
	return l
}

func (s *sni) GetLayout(parent int32, depth int32, names []string) (uint32, menuLayout, *dbus.Error) {
	return 1, s.layout(parent, depth), nil
}

type groupProps struct {
	ID    int32
	Props map[string]dbus.Variant
}

func (s *sni) GetGroupProperties(ids []int32, names []string) ([]groupProps, *dbus.Error) {
	var out []groupProps
	ids2 := ids
	if len(ids2) == 0 {
		for _, it := range menuItems {
			ids2 = append(ids2, it.id)
		}
	}
	for _, id := range ids2 {
		out = append(out, groupProps{id, s.properties(id)})
	}
	return out, nil
}

func (s *sni) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	p := s.properties(id)
	if v, ok := p[name]; ok {
		return v, nil
	}
	return dbus.MakeVariant(""), nil
}

func (s *sni) Event(id int32, eventID string, data dbus.Variant, ts uint32) *dbus.Error {
	if eventID != "clicked" {
		return nil
	}
	for _, it := range menuItems {
		if it.id != id {
			continue
		}
		switch it.special {
		case "auto":
			_ = os.Remove(offPath())
			_ = exec.Command(bin("dualsense-profile-set"), "auto").Run()
		case "off":
			_ = os.MkdirAll(filepath.Dir(offPath()), 0o755)
			_ = os.WriteFile(offPath(), []byte{}, 0o644)
			_ = exec.Command(bin("dualsense-profile-default")).Run()
		case "sep":
		default:
			if it.prof != "" {
				_ = os.Remove(offPath())
				_ = exec.Command(bin("dualsense-profile-set"), it.prof).Run()
			}
		}
	}
	s.refresh()
	return nil
}

func (s *sni) AboutToShow(id int32) (bool, *dbus.Error) { return true, nil }

func (s *sni) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	return []int32{}, []int32{}, nil
}

func (s *sni) mGet(iface, prop string) (dbus.Variant, *dbus.Error) {
	switch prop {
	case "Version":
		return dbus.MakeVariant(uint32(3)), nil
	case "TextDirection":
		return dbus.MakeVariant("ltr"), nil
	case "Status":
		return dbus.MakeVariant("normal"), nil
	case "IconThemePath":
		return dbus.MakeVariant([]string{}), nil
	}
	return dbus.MakeVariant(""), nil
}

func (s *sni) mGetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	m := map[string]dbus.Variant{}
	for _, p := range []string{"Version", "TextDirection", "Status", "IconThemePath"} {
		v, _ := s.mGet(iface, p)
		m[p] = v
	}
	return m, nil
}

// --- регистрация ---

func (s *sni) register() error {
	obj := s.conn.Object(watcher, watcherPath)
	return obj.Call(watcher+".RegisterStatusNotifierItem", 0, s.busName).Err
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

	s := &sni{conn: conn, busName: name}
	conn.Export(s, itemPath, itemIface)
	conn.Export(s, itemPath, propIface)
	conn.Export(s, menuPath, menuIface)
	conn.Export(s, menuPath, propIface)

	if err := s.register(); err != nil {
		fmt.Fprintln(os.Stderr, "register:", err)
	}

	t := time.NewTicker(time.Second)
	defer t.Stop()
	r := time.NewTicker(30 * time.Second)
	defer r.Stop()

	last := s.title()
	for {
		select {
		case <-t.C:
			if s.title() != last {
				last = s.title()
				s.refresh()
			}
		case <-r.C:
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
