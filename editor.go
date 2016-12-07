package jd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/nsf/termbox-go"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	darkGray  = 0xe9 + 2
	gray      = 0xe9 + 7
	lightGray = 0xe9 + 12
	highlight = 0x11 + 60 | termbox.AttrBold
	hintColor = 0x11 + 153 | termbox.AttrBold

	statusBar = gray
	editBar   = gray //53
)
const maxUndos = 10

type Editor struct {
	editdirty    bool
	json         []byte
	root         gjson.Result
	result       gjson.Result
	invalid      bool
	hintkeys     []hintkey
	hintel       gjson.Result
	fullhintpath string
	jsonlines    []int
	prompt       string
	path         string
	editval      string
	eidx         int
	debug        string
	pidx         int // user path cursor index
	hintline     int // the index of line in the hintbox
	topbarsdrawn bool
	parts        []string
	esc          bool
	editmode     bool
	fg, bg       termbox.Attribute
	vpathels     map[string]gjson.Result
	statusy      int
	res1x, res1y int
	res2x, res2y int
	barval       string
	w, h         int
	x, y         int
	writemode    bool
	cx, cy       int // cursor position
	scrolly      int // the ideal scroll position
	resy         int
	undos        []Editor
	undoidx      int
	writeval     string
	widx         int
	perm         os.FileMode
	writeerr     error
	writets      time.Time
}

type hintkey struct {
	key gjson.Result
	val gjson.Result
}
type hintbykey []hintkey

func (arr hintbykey) Len() int {
	return len(arr)
}
func (arr hintbykey) Less(a, b int) bool {
	return arr[a].key.String() < arr[b].key.String()
}

func (arr hintbykey) Swap(a, b int) {
	arr[a], arr[b] = arr[b], arr[a]
}
func Exec(path string) error {
	var b []byte
	var perm os.FileMode = 0600
	var fpath string
	if path == "-" {
		var err error
		b, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	} else if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		fs, err := f.Stat()
		if err != nil {
			return err
		}
		perm = fs.Mode()
		b, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		f.Close()
		fpath = path
	}
	e := &Editor{
		json:     b,
		vpathels: make(map[string]gjson.Result),
		perm:     perm,
		writeval: fpath,
	}
	return e.runloop()
}

func (e *Editor) reflow() {
	var w int

	if len(e.json) > 1*1024*1024 {
		e.resetcolors()
		termbox.Clear(e.bg, e.fg)
		e.x, e.y = 0, 0
		e.blitstr(fmt.Sprintf("loading %d MB buffer", len(e.json)/1024/1024))
		e.newline()
		e.blitstr("please wait...")
		termbox.Flush()
	}

	w, e.h = termbox.Size()
	if w != e.w || e.editdirty {
		e.w = w
		pjson := pretty(e.json, e.w)
		e.root = gjson.Parse(string(pjson))
		e.vpathels = make(map[string]gjson.Result)
		e.countjsonlines()
		e.editdirty = false
	}
	e.exec()
	e.redraw()
}

func (e *Editor) countjsonlines() {
	e.jsonlines = e.jsonlines[:0]
	var i int
	var x int
	e.jsonlines = append(e.jsonlines, 0)
	for ; i < len(e.root.Raw); i++ {
		if e.root.Raw[i] == '\n' {
			x = 0
			e.jsonlines = append(e.jsonlines, i+1)
			continue
		} else if x == e.w {
			x = 0
			e.jsonlines = append(e.jsonlines, i)
			continue
		}
		x++
	}
}

func (e *Editor) blitstr(s string) {
	for _, c := range s {
		if c == '\n' {
			e.x = 0
			e.y++
			if e.y > e.h {
				return
			}
			continue
		}
		if e.y <= e.h && !e.topbarsdrawn || e.y >= e.resy {
			termbox.SetCell(e.x, e.y, c, e.fg, e.bg)
		}
		e.x++
		if e.x == e.w {
			e.newline()
		}
	}
}

func (e *Editor) redraw() {
	e.resetcolors()
	e.topbarsdrawn = false
	e.x, e.y = 0, 0
	e.w, e.h = termbox.Size()
	termbox.Clear(e.bg, e.fg)
	e.blitstr(e.prompt)
	e.blitpath()
	e.blitstatus()
	e.topbarsdrawn = true
	e.blitres()
	e.blitdebug()
	e.blithelp()
	e.bliterr()
	e.blitcursor()
	termbox.Flush()
}
func (e *Editor) bliterr() {
	if e.writets.IsZero() || e.writeerr == nil || time.Now().Sub(e.writets) > time.Second*3 {
		e.writeerr = nil
		e.writets = time.Time{}
		return
	}
	errmsg := "[ " + e.writeerr.Error() + " ]"
	errmsg = e.centerstr(errmsg)

	y := e.h - 3
	x := 0

	for _, c := range errmsg {
		if c == '[' {
			break
		}
		termbox.SetCell(x, y, ' ', termbox.ColorDefault, termbox.ColorDefault)
		x++
	}
	var in bool
	for _, c := range errmsg {
		if c == '[' {
			in = true
		}
		if in {
			termbox.SetCell(x, y, c, termbox.ColorBlack, termbox.ColorWhite)
			x++
		}
	}
}

func (e *Editor) blithelp() {
	x := 0
	mx := func() int {
		px := x
		x++
		return px
	}
	ps := func(h, s string) {
		for _, c := range h {
			termbox.SetCell(mx(), e.h-1, c, termbox.ColorBlack, termbox.ColorWhite)
		}
		termbox.SetCell(mx(), e.h-1, ' ', termbox.ColorDefault, termbox.ColorDefault)
		for _, c := range s {
			termbox.SetCell(mx(), e.h-1, c, termbox.ColorDefault, termbox.ColorDefault)
		}
		x += 3
	}
	for x := 0; x < e.w; x++ {
		termbox.SetCell(x, e.h-1, ' ', termbox.ColorDefault, termbox.ColorDefault)
	}
	if e.writemode {
		ps("^C", "Cancel")
	} else {
		ps("^X", "Exit")
		ps("^E", "Edit")
		ps("^D", "Delete")
		ps("^O", "WriteOut")
		ps("^Z", "Undo")
	}
}
func (e *Editor) blitcursor() {
	if e.editmode {
		y := e.eidx/e.w + e.statusy + 1
		x := e.eidx % e.w
		termbox.SetCursor(x, y)
	} else {
		y := e.pidx / e.w
		x := e.pidx % e.w
		termbox.SetCursor(x, y)
	}
}

func (e *Editor) newline() {
	e.x = 0
	e.y++
}

func (e *Editor) blitdebug() {
	if e.debug == "" {
		return
	}
	e.newline()
	if e.y >= e.h {
		e.y = e.h - 1
	}
	e.bg = darkGray
	e.fg = termbox.ColorWhite
	lines := strings.Split(e.debug, "\n")
	for _, line := range lines {
		if len(line) > e.w {
			line = line[:e.w]
		} else {
			line = line + strings.Repeat(" ", e.w-len(line))
		}
		yy := e.y
		e.y = e.h - 2
		e.blitstr(line)
		e.y = yy
	}
	e.resetcolors()
}

// scrollto scrolls the result to the specific byte position.
func (e *Editor) scrollintoview(pos, count int) {
	defer func() {
		e.y = e.resy - e.scrolly
	}()
	vislines := e.h - e.resy - 1
	if vislines < 1 {
		vislines = 1
	}
	if len(e.jsonlines) <= vislines {
		e.scrolly = 0
		// we have enough room to store the entire buffer
		return
	}
	var sline int
	// get the line for the pos
	var i int

	for ; i < len(e.jsonlines); i++ {
		sline = i
		if pos <= e.jsonlines[i] {
			i++
			if sline > 0 {
				sline--
			}
			break
		}
	}
	eline := sline + 1
	for ; i < len(e.jsonlines); i++ {
		eline = i
		if pos+count <= e.jsonlines[i] {
			if eline > 0 {
				eline--
			}
			break
		}
	}

	// sline = the starting line of the element to scroll to
	// eline = the ending line of the element to scroll to
	// vislines = the number of visible lines on screen
	// e.scrolly = the current scroll position

	// is the entire selection already visible?
	if eline-sline <= vislines && sline >= e.scrolly && eline <= e.scrolly+vislines {
		// the entire is already on screen
	} else {
		e.scrolly = sline - 1
	}
	if e.scrolly < 0 {
		e.scrolly = 0
	}
	if e.scrolly > len(e.jsonlines)-vislines {
		e.scrolly = len(e.jsonlines) - vislines
	}
}

func (e *Editor) blitres() {
	e.resy = e.y
	e.fg = lightGray
	defer e.resetcolors()
	if e.invalid || e.result.Index == 0 {
		if len(e.hintkeys) > 0 && e.hintel.Type == gjson.JSON { //&& e.hintel.Raw[0] == '{' {
			s := 0
			idx := e.hintline % len(e.hintkeys)
			if idx < 0 {
				idx = len(e.hintkeys) + idx
			}
			hkey := e.hintkeys[idx]
			var hres gjson.Result
			if e.hintel.Raw[0] == '[' {
				hres = hkey.val
			} else {
				hres = hkey.key
			}
			e.scrollintoview(hres.Index, len(hres.Raw))
			e.blitstr(e.root.Raw[s:hres.Index])
			s = hres.Index + len(hres.Raw)
			e.fg = hintColor
			e.res1x, e.res1y = e.x, e.y
			e.blitstr(e.root.Raw[hres.Index:s])
			e.res2x, e.res2y = e.x, e.y
			e.fg = lightGray
			e.blitstr(e.root.Raw[s:])
		} else {
			e.res1x, e.res1y = e.x, e.y
			e.blitstr(e.root.Raw)
			e.res2x, e.res2y = 0, 0
		}
		return
	}
	e.scrollintoview(e.result.Index, len(e.result.Raw))
	e.blitstr(e.root.Raw[:e.result.Index])
	e.fg = highlight
	e.res1x, e.res1y = e.x, e.y
	e.blitstr(e.root.Raw[e.result.Index : e.result.Index+len(e.result.Raw)])
	e.res2x, e.res2y = e.x, e.y
	e.fg = lightGray
	e.blitstr(e.root.Raw[e.result.Index+len(e.result.Raw):])
}

func (e *Editor) blitpath() {
	if e.invalid && len(e.hintkeys) > 0 {
		ukey := e.parts[len(e.parts)-1]
		e.blitstr(e.path)
		e.fg = hintColor
		idx := e.hintline % len(e.hintkeys)
		if idx < 0 {
			idx = len(e.hintkeys) + idx
		}
		exkey := e.hintkeys[idx].key.String()[len(ukey):]
		cx, cy := e.x, e.y
		e.blitstr(exkey)
		e.x, e.y = cx, cy
		e.fullhintpath = e.path + exkey
	} else {
		e.fullhintpath = ""
		e.blitstr(e.path)
	}
}

func (e *Editor) centerstr(s string) string {
	x := (e.w - len(s)) / 2
	if x == 0 {
		return s
	}
	if x < 0 {
		return s[x:]
	}
	return strings.Repeat(" ", x) + s
}

func (e *Editor) blitstatus() {
	e.statusy = e.y
	e.barval = ""
	var barstr string
	var isobj bool
	if e.editmode {
		barstr = e.editval
	} else {
		if !e.invalid {
			if e.result.Type == gjson.JSON {
				isobj = true
				if e.result.Raw[0] == '{' {
					//barstr = fmt.Sprintf("{...}")
				} else {
					//barstr = fmt.Sprintf("[...]")
				}
			} else {
				if e.result.Type == gjson.Number {
					barstr = e.result.String()
				} else {
					barstr = e.result.Raw
				}
			}
		} else {
			if e.hintel.Exists() && len(e.hintkeys) > 0 {
				idx := e.hintline % len(e.hintkeys)
				if idx < 0 {
					idx = len(e.hintkeys) + idx
				}
				hkey := e.hintkeys[idx]
				if hkey.val.Exists() {
					if hkey.val.Type == gjson.JSON {
						isobj = true
						if hkey.val.Raw[0] == '{' {
							//barstr = fmt.Sprintf("{...}")
						} else {
							//barstr = fmt.Sprintf("[...]")
						}
					} else {
						barstr = hkey.val.Raw
					}
				}
			} else {
				barstr = ""
			}
		}
		e.barval = barstr
	}
	if e.editmode {
		e.bg = editBar
		e.fg = termbox.ColorWhite | termbox.AttrBold
	} else {
		e.bg = statusBar
		if !isobj {
			e.fg = termbox.ColorWhite | termbox.AttrBold
		} else {
			e.fg = lightGray + 3
		}

	}
	if len(barstr) > e.w {
		barstr = barstr[:e.w]
	} else {
		barstr += strings.Repeat(" ", e.w-len(barstr))
	}
	e.newline()
	e.blitstr(barstr)
	e.resetcolors()
}

func (e *Editor) resetcolors() {
	e.bg, e.fg = termbox.ColorDefault, termbox.ColorDefault
}
func (e *Editor) exechints() {
	e.hintkeys = nil
	e.hintel = gjson.Result{}
	if !e.invalid || e.esc {
		return
	}
	e.hintel = e.root
	if len(e.parts) > 1 {
		vpath := strings.Join(e.parts[:len(e.parts)-1], ".")
		if el, ok := e.vpathels[vpath]; ok {
			e.hintel = el
		} else {
			e.hintel = gjson.Get(e.root.Raw, vpath)
			e.vpathels[vpath] = e.hintel
		}
	}
	if e.hintel.Type == gjson.JSON {
		var keys []hintkey
		var num float64
		e.hintel.ForEach(func(key, val gjson.Result) bool {
			if e.hintel.Raw[0] == '[' {
				key = gjson.Result{Type: gjson.Number, Num: num}
			}
			if strings.HasPrefix(key.String(), e.parts[len(e.parts)-1]) {
				if e.hintel.Raw[0] == '{' {
					key.Index += e.hintel.Index
				}
				val.Index += e.hintel.Index
				keys = append(keys, hintkey{key, val})
			}
			num++
			return true
		})
		e.hintkeys = keys
	}
}

func (e *Editor) exec() {
	defer func() {
		e.exechints()
	}()

	e.parts, e.esc = parsePath(e.path)
	if e.path == "" {
		e.result = e.root
		e.invalid = true
		return
	}
	res := gjson.Get(e.root.Raw, e.path)
	if !res.Exists() {
		e.invalid = true
		return
	}
	e.invalid = false
	e.result = res
}

func parsePath(path string) (parts []string, esc bool) {
	var i int
	var s int
	for ; i < len(path); i++ {
		if path[i] == '\\' {
			esc = true
		}
		if path[i] == '.' {
			j := i - 1
			var sc int
			for ; j > s; j-- {
				if path[j] == '\\' {
					sc++
				} else {
					break
				}
			}
			if sc%2 == 0 {
				parts = append(parts, path[s:i])
				s = i + 1
			}
		}
	}
	parts = append(parts, path[s:i])
	if esc {
		for i := range parts {

			vparts := strings.Split(parts[i], "\\\\")
			for i := range vparts {
				vparts[i] = strings.Replace(vparts[i], "\\", "", -1)
			}
			parts[i] = strings.Join(vparts, "\\")
		}
	}
	return parts, esc
}

func (e *Editor) addrune(r rune) {
	if e.editmode {
		if e.eidx >= len(e.editval) {
			e.editval += string(r)
			e.eidx = len(e.editval)
		} else {
			e.editval = e.editval[:e.eidx] + string(r) + e.editval[e.eidx:]
			e.eidx++
		}
	} else {
		if e.pidx >= len(e.path) {
			e.path += string(r)
			e.pidx = len(e.path)
		} else {
			e.path = e.path[:e.pidx] + string(r) + e.path[e.pidx:]
			e.pidx++
		}
		e.hintline = 0
	}
	e.exec()
	e.redraw()
}

func (e *Editor) completeedit() {
	var njson []byte
	var err error
	if valid(e.editval) {
		njson, err = sjson.SetRawBytes(e.json, e.path, []byte(e.editval))
	} else {
		njson, err = sjson.SetBytes(e.json, e.path, e.editval)
	}
	if err != nil {
		e.writeerr = err
		e.writets = time.Now()
	} else {
		e.undos = append(e.undos, *e)
		if len(e.undos) > maxUndos {
			e.undos = e.undos[len(e.undos)-maxUndos:]
		} else {
			e.undoidx++
		}
		e.json = njson
	}
	e.editmode = false
	e.editdirty = true
	e.reflow()
}

func (e *Editor) delete() {
	ppath := e.path
	ppidx := e.pidx
	var njson []byte
	e.completehint(false)
	if e.path == "" {
		if e.root.Type != gjson.JSON || len(e.json) == 2 {
			njson = []byte("")
		}
	} else {
		var err error
		njson, err = sjson.DeleteBytes(e.json, e.path)
		if err != nil {
			e.writeerr = err
			e.writets = time.Now()
		}
	}
	e.undos = append(e.undos, *e)
	if len(e.undos) > maxUndos {
		e.undos = e.undos[len(e.undos)-maxUndos:]
	} else {
		e.undoidx++
	}
	e.json = njson
	e.path = ppath
	e.pidx = ppidx
	e.editmode = false
	e.editdirty = true
	e.reflow()
}

// completehint will fill-in the partial hint path
func (e *Editor) completehint(adddot bool) {
	e.hintline = 0
	if e.fullhintpath != "" {
		e.path = e.fullhintpath
	}
	e.pidx = len(e.path)
	e.exec()
	if adddot {
		if !e.invalid && e.result.Type == gjson.JSON {
			e.path += "."
			e.pidx++
			e.exec()
		}
	}
	e.redraw()
}
func (e *Editor) undo() {
	if e.undoidx > 0 {
		*e = e.undos[e.undoidx-1]
		e.redraw()
	}
}
func (e *Editor) redo() {
}

func (e *Editor) writeOut() {
	e.widx = len(e.writeval)
	e.writemode = true
	e.writeredraw()
}

func (e *Editor) addwriterune(c rune) {
	e.writeval += string(c)
	e.widx++
	e.writeredraw()
}
func (e *Editor) writeredraw() {
	for x := 0; x < e.w; x++ {
		termbox.SetCell(x, e.h-2, ' ', termbox.ColorBlack, termbox.ColorWhite)
	}
	prompt := "File Name to Write: "
	x := 0
	for _, c := range prompt {
		termbox.SetCell(x, e.h-2, c, termbox.ColorBlack, termbox.ColorWhite)
		x++
	}
	for _, c := range e.writeval {
		termbox.SetCell(x, e.h-2, c, termbox.ColorBlack, termbox.ColorWhite)
		x++
	}
	termbox.SetCursor(x-(len(e.writeval)-e.widx), e.h-2)
	e.blithelp()
	e.bliterr()
	termbox.Flush()
}
func (e *Editor) completewrite() {
	e.writets = time.Now()
	if err := ioutil.WriteFile(e.writeval, e.json, e.perm); err != nil {
		e.writeerr = err
		e.writeredraw()
		return
	}
	e.writemode = false
	e.writeerr = errors.New("written")
	e.redraw()
}
func (e *Editor) cancelwrite() {
	e.writemode = false
	e.writeerr = nil
	e.writets = time.Time{}
	e.redraw()
}

// runloop runs the engine
func (e *Editor) runloop() error {
	if err := termbox.Init(); err != nil {
		return err
	}
	defer termbox.Close()
	termbox.SetOutputMode(termbox.Output256)
	e.reflow()

	for {
		if e.writemode {
			switch ev := termbox.PollEvent(); ev.Type {
			case termbox.EventKey:
				switch ev.Key {
				default:
					if ev.Ch == 0 && ev.Key == 3 {
						// Ctrl-C
						e.cancelwrite()
						break
					}
					if ev.Ch != 0 {
						e.addwriterune(ev.Ch)
					}
				case termbox.KeyBackspace, termbox.KeyBackspace2:
					if len(e.writeval) > 0 {
						if e.widx >= len(e.writeval) {
							e.writeval = e.writeval[:len(e.writeval)-1]
							e.widx = len(e.writeval)
						} else if e.widx > 0 {
							e.writeval = e.writeval[:e.widx-1] + e.writeval[e.widx:]
							e.widx--
						}
					}
					e.writeredraw()
				case termbox.KeyArrowLeft:
					e.widx--
					if e.widx < 0 {
						e.widx = 0
					}
					e.writeredraw()
				case termbox.KeyArrowRight:
					e.widx++
					if e.widx >= len(e.writeval) {
						e.widx = len(e.writeval)
					}
					e.writeredraw()
				case termbox.KeyEnd:
					e.widx = len(e.writeval)
					e.writeredraw()
				case termbox.KeyHome:
					e.widx = 0
					e.writeredraw()
				case termbox.KeySpace:
					e.addwriterune(' ')
				case termbox.KeyEnter:
					e.completewrite()
				}
			}
			continue
		}
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			default:
				if ev.Ch == 0 && ev.Key == 15 {
					e.writeOut()
					break
					//return nil // Ctrl-C, exit
				}
				if ev.Ch == 0 && ev.Key == 4 {
					e.delete()
					break
					//return nil // Ctrl-C, exit
				}
				if ev.Ch == 0 && ev.Key == 3 {
					break
					//return nil // Ctrl-C, exit
				}
				if ev.Ch == 0 && ev.Key == 24 {
					return nil // Ctrl-X, exit
				}
				if ev.Ch == 0 && ev.Key == 25 {
					// Ctrl-Y, redo
					//e.redo()
					break
				}
				if ev.Ch == 0 && ev.Key == 26 {
					// Ctrl-Z, redo
					e.undo()
					break
				}
				if ev.Ch == 0 && ev.Key == 5 {
					// Ctrl-E, edit
					if e.editmode {
						e.editmode = false
						e.exec()
						e.redraw()
					} else {
						e.completehint(false)
						e.editmode = true
						e.editval = e.barval
						e.eidx = len(e.editval)
						e.exec()
						e.redraw()
					}
					break
				}
				if ev.Ch != 0 {
					e.addrune(ev.Ch)
				}
			case termbox.KeyBackspace, termbox.KeyBackspace2:
				if e.editmode {
					if len(e.editval) > 0 {
						if e.eidx >= len(e.editval) {
							e.editval = e.editval[:len(e.editval)-1]
							e.eidx = len(e.editval)
						} else if e.eidx > 0 {
							e.editval = e.editval[:e.eidx-1] + e.editval[e.eidx:]
							e.eidx--
						}
						e.exec()
						e.redraw()
					}
				} else {
					if len(e.path) > 0 {
						if e.pidx >= len(e.path) {
							e.path = e.path[:len(e.path)-1]
							e.pidx = len(e.path)
						} else if e.pidx > 0 {
							e.path = e.path[:e.pidx-1] + e.path[e.pidx:]
							e.pidx--
						}
						e.exec()
						e.redraw()
					}
				}
			case termbox.KeyArrowLeft:
				if e.editmode {
					e.eidx--
					if e.eidx < 0 {
						e.eidx = 0
					}
				} else {
					e.pidx--
					if e.pidx < 0 {
						e.pidx = 0
					}
				}
				e.redraw()
			case termbox.KeyArrowRight:
				e.pidx++
				if e.pidx >= len(e.path) {
					e.pidx = len(e.path)
				}
				e.redraw()
			case termbox.KeyEnd:
				if e.editmode {
					e.eidx = len(e.editval)
				} else {
					e.pidx = len(e.path)
				}
				e.redraw()
			case termbox.KeyHome:
				if e.editmode {
					e.eidx = 0
				} else {
					e.pidx = 0
				}
				e.redraw()
			case termbox.KeySpace:
				e.addrune(' ')
			case termbox.KeyTab, termbox.KeyEnter:
				if e.editmode {
					e.completeedit()
				} else {
					e.completehint(true)
				}
			case termbox.KeyArrowDown:
				e.hintline++
				e.exec()
				e.redraw()
			case termbox.KeyArrowUp:
				e.hintline--
				e.exec()
				e.redraw()
			case termbox.KeyEsc:
				if e.editmode {
					e.editmode = false
					e.exec()
					e.redraw()
				}
			}
		case termbox.EventResize:
			e.reflow()
		}
	}
}
