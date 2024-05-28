package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
)

var logpath string
var logname = "clockon.log"

func init() {
	cd, _ := os.UserCacheDir()
	logpath = filepath.Join(cd, "clockon")
}

type entry struct {
	a   string
	typ state
	t   time.Time
	d   time.Duration
}

func (e entry) String() string {
	switch e.typ {
	case selecting:
		return fmt.Sprintf("%s\nc %s\n", e.a, e.t.Format(time.RFC3339))
	case removing:
		return fmt.Sprintf("%s\nd %s\n", e.a, e.t.Format(time.RFC3339))
	case working:
		return fmt.Sprintf("%s\nw %s %s\n", e.a, e.t.Format(time.RFC3339), e.d.Round(time.Second))
	case resting:
		return fmt.Sprintf("%s\nb %s %s\n", e.a, e.t.Format(time.RFC3339), e.d.Round(time.Second))
	}
	return ""
}

type logger struct {
	sidx     int
	bidx     int
	bread    bool
	session  []entry
	buffered []entry
}

func newlogger() (*logger, error) {
	os.MkdirAll(logpath, os.ModeDir)
	return &logger{
		session: make([]entry, 0, 50),
	}, nil
}

func (l *logger) send(e entry) {
	if e.d > 0 {
		e.t = e.t.Add(e.d * -1)
	}
	l.session = append(l.session, e)
}

func (l *logger) flush() {
	f, err := os.OpenFile(filepath.Join(logpath, logname), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	for _, e := range l.session {
		f.WriteString(e.String())
	}
	f.Close()
}

func sameDay(a, b time.Time) bool {
	if a.Day() == b.Day() && a.Month() == b.Month() && a.Year() == b.Year() {
		return true
	}
	return false
}

func zero(buf []entry) []entry {
	for i := range buf {
		buf[i].t = time.Time{}
		buf[i].d = 0
	}
	return buf
}

func (l *logger) shrink(activities []string) {
	l.bidx = 0
	l.sidx = 0
	f, err := os.Create(filepath.Join(logpath, logname))
	if err != nil {
		return
	}
	actIdx := make(map[string]int)
	for i, v := range activities {
		actIdx[v] = i * 2
	}
	buffer := make([]entry, len(activities)*2)
	for k, v := range actIdx {
		buffer[v].a = k
		buffer[v].typ = working
		buffer[v+1].a = k
		buffer[v+1].typ = resting
	}
	var this time.Time
	for e, err := l.next(); ; e, err = l.next() {
		if !sameDay(this, e.t) {
			for _, v := range buffer {
				if v.d > 0 {
					f.WriteString(v.String())
				}
			}
			if err != nil { // we are done!
				break
			}
			buffer = zero(buffer)
			this = e.t
		}
		if e.typ != working && e.typ != resting {
			continue
		}
		idx, ok := actIdx[e.a]
		if !ok {
			continue
		}
		if e.typ == resting {
			idx += 1
		}
		buffer[idx].t = e.t
		buffer[idx].d += e.d
	}
	f.Close()
}

func (l *logger) next() (entry, error) {
	if !l.bread {
		l.bread = true
		f, err := os.Open(filepath.Join(logpath, logname))
		if err != nil {
			return entry{}, err
		}
		es, err := loadAll(f)
		f.Close()
		if err != nil {
			return entry{}, err
		}
		l.buffered = es
	}
	if l.bidx < len(l.buffered) {
		l.bidx += 1
		return l.buffered[l.bidx-1], nil
	}
	if l.sidx < len(l.session) {
		l.sidx += 1
		return l.session[l.sidx-1], nil
	}
	return entry{}, io.EOF
}

func (l *logger) prev() (entry, error) {
	if !l.bread {
		l.bread = true
		f, err := os.Open(filepath.Join(logpath, logname))
		if err != nil {
			return entry{}, err
		}
		es, err := loadAll(f)
		f.Close()
		if err != nil {
			return entry{}, err
		}
		l.buffered = es
	}
	if l.sidx < len(l.session) {
		l.sidx += 1
		return l.session[len(l.session)-l.sidx], nil
	}
	if l.bidx < len(l.buffered) {
		l.bidx += 1
		return l.buffered[len(l.buffered)-l.bidx], nil
	}
	return entry{}, io.EOF
}

func loadAll(f *os.File) ([]entry, error) {
	ret := make([]entry, 0, 1000)
	s := bufio.NewScanner(f)
	var err error
	for e, err := load(s); err == nil; e, err = load(s) {
		ret = append(ret, e)
	}
	if err == io.EOF {
		return ret, nil
	}
	return ret, err
}

func load(s *bufio.Scanner) (entry, error) {
	if !s.Scan() {
		err := s.Err()
		if err == nil {
			err = io.EOF
		}
		return entry{}, err
	}
	e := entry{
		a: s.Text(),
	}
	if !s.Scan() {
		err := s.Err()
		if err == nil {
			err = io.EOF
		}
		return entry{}, err
	}
	triplet := strings.SplitN(s.Text(), " ", 3)
	if len(triplet) < 1 || len(triplet[0]) != 1 {
		return entry{}, errors.New("bad entry")
	}
	switch triplet[0] {
	case "c":
		e.typ = selecting
	case "d":
		e.typ = removing
	case "w":
		e.typ = working
	case "b":
		e.typ = resting
	default:
		return entry{}, errors.New("bad entry")
	}
	if e.typ == selecting || e.typ == removing {
		return e, nil
	}
	et, err := time.Parse(time.RFC3339, triplet[1])
	if err != nil {
		return entry{}, err
	}
	e.t = et
	ed, err := time.ParseDuration(triplet[2])
	if err != nil {
		return entry{}, err
	}
	e.d = ed
	return e, nil
}

func (l *logger) tally(activities []string) [][2]time.Duration {
	l.bidx = 0
	l.sidx = 0
	ret := make([][2]time.Duration, len(activities))
	y, m, d := time.Now().Date()
	for e, err := l.prev(); err == nil; e, err = l.prev() {
		yy, mm, dd := e.t.Date()
		if y != yy || m != mm || d != dd {
			break
		}
		if e.typ != working && e.typ != resting {
			continue
		}
		for i, v := range activities {
			if v != e.a {
				continue
			}
			if e.typ == working {
				ret[i][0] += e.d
				break
			}
			ret[i][1] += e.d
			break
		}
	}
	return ret
}

// returns current activities, currently selected activity, current and previous week (reports), current and previous year (reports)
func (l *logger) refresh() ([]string, int, [2]int, [2]int, int, int) {
	l.bidx, l.sidx = 0, 0
	scratch := make(map[string]struct{})
	for e, err := l.next(); err == nil; e, err = l.next() {
		if e.typ == removing {
			delete(scratch, e.a)
			continue
		}
		if _, ok := scratch[e.a]; ok {
			continue
		}
		scratch[e.a] = struct{}{}
	}
	// now get the most recently selected
	l.bidx, l.sidx = 0, 0
	var this string
	for e, err := l.prev(); err == nil; e, err = l.prev() {
		if e.typ == removing {
			continue
		}
		if _, ok := scratch[e.a]; ok {
			this = e.a
			break
		}
	}
	ret := make([]string, len(scratch))
	idx, ridx := 0, 0
	for k := range scratch {
		ret[idx] = k
		idx++
	}
	sort.Strings(ret)
	for i, v := range ret {
		if v == this {
			ridx = i
			break
		}
	}
	// now get the weeks and years
	var thisWk, prevWk [2]int
	var prevYr int
	l.bidx, l.sidx = 0, 0
	for e, err := l.prev(); err == nil; e, err = l.prev() {
		if e.typ != working && e.typ != resting {
			continue
		}
		if _, ok := scratch[e.a]; !ok {
			continue
		}
		yr, wk := e.t.ISOWeek()
		if thisWk[0] == 0 {
			thisWk[0], thisWk[1] = yr, wk
			continue
		}
		if prevWk[0] == 0 && (wk < thisWk[1] || yr < thisWk[0]) {
			prevWk[0], prevWk[1] = yr, wk
		}
		if prevYr == 0 && yr < thisWk[0] {
			prevYr = thisWk[0]
			break
		}
	}

	return ret, ridx, thisWk, prevWk, thisWk[0], prevYr
}

func (l *logger) active() map[string][]entry {
	l.bidx = 0
	l.sidx = 0
	ret := make(map[string][]entry)
	for e, err := l.next(); err != nil; e, err = l.next() {
		if e.typ == removing {
			delete(ret, e.a)
			continue
		}
		if _, ok := ret[e.a]; !ok {
			ret[e.a] = make([]entry, 0, 50)
		}
		ret[e.a] = append(ret[e.a], e)
	}
	return ret
}

func (l *logger) history() map[string][]entry {
	l.bidx = 0
	l.sidx = 0
	ret := make(map[string][]entry)
	for e, err := l.next(); err != nil; e, err = l.next() {
		if _, ok := ret[e.a]; !ok {
			ret[e.a] = make([]entry, 0, 50)
		}
		ret[e.a] = append(ret[e.a], e)
	}
	return ret
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh%02dm", h, m)
}

func toRows(d [][2]time.Duration) []table.Row {
	ret := make([]table.Row, 3)
	ret[0] = make(table.Row, len(d)+2)
	ret[1] = make(table.Row, len(d)+2)
	ret[2] = make(table.Row, len(d)+2)
	totals := [2]time.Duration{}
	ret[0][0] = "work"
	ret[1][0] = "break"
	ret[2][0] = "total"
	for i, v := range d {
		ret[0][i+1] = fmtDuration(v[0].Round(time.Minute))
		ret[1][i+1] = fmtDuration(v[1].Round(time.Minute))
		ret[2][i+1] = fmtDuration((v[0] + v[1]).Round(time.Minute))
		totals[0] += v[0]
		totals[1] += v[1]
	}
	ret[0][len(ret[0])-1] = fmtDuration(totals[0].Round(time.Minute))
	ret[1][len(ret[1])-1] = fmtDuration(totals[1].Round(time.Minute))
	ret[2][len(ret[2])-1] = fmtDuration((totals[0] + totals[1]).Round(time.Minute))
	return ret
}

func dayIndex(t time.Time) int {
	day := t.Weekday()
	if day == 0 {
		day = 7
	}
	return int(day) - 1
}

func (l *logger) weeks(activity string, week [2]int) ([]table.Row, [2]int, [2]int) {
	l.bidx = 0
	l.sidx = 0
	var nxt, prev [2]int
	d := make([][2]time.Duration, 7)
	for e, err := l.next(); err == nil; e, err = l.next() {
		if e.typ != working && e.typ != resting {
			continue
		}
		if e.a != activity {
			continue
		}
		thisYr, thisWk := e.t.ISOWeek()
		if thisWk == week[1] && thisYr == week[0] {
			d[dayIndex(e.t)][e.typ-4] += e.d
			continue
		}
		if thisYr < week[0] || (thisYr == week[0] && thisWk < week[1]) {
			prev[0], prev[1] = thisYr, thisWk
			continue
		}
		nxt[0], nxt[1] = thisYr, thisWk
		break
	}
	return toRows(d), nxt, prev
}

func (l *logger) years(activity string, year int) ([]table.Row, int, int) {
	l.bidx = 0
	l.sidx = 0
	var nxt, prev int
	d := make([][2]time.Duration, 12)
	for e, err := l.next(); err == nil; e, err = l.next() {
		if e.typ != working && e.typ != resting {
			continue
		}
		if e.a != activity {
			continue
		}
		thisYr := e.t.Year()
		if thisYr == year {
			d[e.t.Month()-1][e.typ-4] += e.d
			continue
		}
		if thisYr < year {
			prev = thisYr
			continue
		}
		nxt = thisYr
		break
	}
	return toRows(d), nxt, prev
}
