// tarra — tulostaa tekstiä ja ruuvisymboleja D1-tarroille (oletus 12 mm teippi).
//
// Luo PDF:n, jonka sivu on tarran kokoinen, ja lähettää sen macOS:n/CUPS:n
// lp-komennolla tulostimelle. Ei ulkoisia riippuvuuksia.
//
//	tarra "M3x8 :ph:"
//	tarra "M4x20|A2 ruostumaton :hex:"      (| = rivinvaihto)
//	tarra -n "M3x8 :torx:"                  (vain PDF, ei tulosta)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

const ptPerMM = 72.0 / 25.4

// Helvetica / Helvetica-Bold -leveydet (1/1000 em), ASCII 32..126.
var regW = []int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556,
	278, 278, 584, 584, 584, 556, 1015,
	667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611,
	278, 278, 278, 469, 556, 333,
	556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556, 556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500,
	334, 260, 334, 584,
}

var boldW = []int{
	278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556,
	333, 333, 584, 584, 584, 611, 975,
	722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611,
	333, 278, 333, 584, 556, 333,
	556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611, 611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500,
	389, 280, 389, 584,
}

var accentBase = map[rune]rune{}

func init() {
	if len(regW) != 95 || len(boldW) != 95 {
		panic(fmt.Sprintf("leveystaulukon pituus väärä: %d/%d", len(regW), len(boldW)))
	}
	for _, g := range []string{
		"AÀÁÂÃÄÅ", "CÇ", "EÈÉÊË", "IÌÍÎÏ", "NÑ", "OÒÓÔÕÖØ", "UÙÚÛÜ", "YÝ",
		"aàáâãäå", "cç", "eèéêë", "iìíîï", "nñ", "oòóôõöø", "uùúûü", "yýÿ",
	} {
		rs := []rune(g)
		for _, r := range rs[1:] {
			accentBase[r] = rs[0]
		}
	}
}

func glyphWidth(r rune, bold bool) float64 {
	if b, ok := accentBase[r]; ok {
		r = b
	}
	tbl := regW
	if bold {
		tbl = boldW
	}
	if r >= 32 && r <= 126 {
		return float64(tbl[r-32]) / 1000
	}
	return 0.556
}

// cp1252 + oktaalikoodaus PDF-merkkijonoon.
func pdfString(s string) string {
	var b strings.Builder
	for _, r := range s {
		var c byte
		switch {
		case r == '€':
			c = 0x80
		case r >= 32 && r < 127:
			c = byte(r)
		case r >= 0xA0 && r <= 0xFF:
			c = byte(r)
		default:
			c = '?'
		}
		if c == '(' || c == ')' || c == '\\' || c < 32 || c > 126 {
			fmt.Fprintf(&b, "\\%03o", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// ---------- piirtäminen ----------

type canvas struct{ b bytes.Buffer }

func (c *canvas) p(format string, a ...any) { fmt.Fprintf(&c.b, format+"\n", a...) }

func (c *canvas) rect(x, y, w, h float64) { c.p("%.4f %.4f %.4f %.4f re f", x, y, w, h) }

func (c *canvas) rotRect(cx, cy, w, h, deg float64) {
	a := deg * math.Pi / 180
	c.p("q %.4f %.4f %.4f %.4f %.4f %.4f cm %.4f %.4f %.4f %.4f re f Q",
		math.Cos(a), math.Sin(a), -math.Sin(a), math.Cos(a), cx, cy, -w/2, -h/2, w, h)
}

func (c *canvas) circlePath(cx, cy, r float64) {
	k := 0.5522847498 * r
	c.p("%.4f %.4f m", cx+r, cy)
	c.p("%.4f %.4f %.4f %.4f %.4f %.4f c", cx+r, cy+k, cx+k, cy+r, cx, cy+r)
	c.p("%.4f %.4f %.4f %.4f %.4f %.4f c", cx-k, cy+r, cx-r, cy+k, cx-r, cy)
	c.p("%.4f %.4f %.4f %.4f %.4f %.4f c", cx-r, cy-k, cx-k, cy-r, cx, cy-r)
	c.p("%.4f %.4f %.4f %.4f %.4f %.4f c h", cx+k, cy-r, cx+r, cy-k, cx+r, cy)
}

func (c *canvas) ring(cx, cy, r, lw float64) {
	c.p("%.4f w", lw)
	c.circlePath(cx, cy, r)
	c.p("S")
}

func (c *canvas) poly(pts [][2]float64, op string) {
	for i, p := range pts {
		if i == 0 {
			c.p("%.4f %.4f m", p[0], p[1])
		} else {
			c.p("%.4f %.4f l", p[0], p[1])
		}
	}
	c.p("h %s", op)
}

func regPoly(cx, cy, r float64, n int, startDeg float64) [][2]float64 {
	pts := make([][2]float64, n)
	for i := range pts {
		a := (startDeg + 360*float64(i)/float64(n)) * math.Pi / 180
		pts[i] = [2]float64{cx + r*math.Cos(a), cy + r*math.Sin(a)}
	}
	return pts
}

func star(cx, cy, ro, ri float64, n int, startDeg float64) [][2]float64 {
	pts := make([][2]float64, 2*n)
	for i := range pts {
		r := ro
		if i%2 == 1 {
			r = ri
		}
		a := (startDeg + 180*float64(i)/float64(n)) * math.Pi / 180
		pts[i] = [2]float64{cx + r*math.Cos(a), cy + r*math.Sin(a)}
	}
	return pts
}

type symbol struct {
	names []string
	desc  string
	draw  func(c *canvas, x, y, s float64)
}

func headRing(c *canvas, x, y, s float64) {
	c.ring(x+.5*s, y+.5*s, .44*s, .11*s)
}

var symbols = []symbol{
	{[]string{"ph", "risti"}, "ristipääruuvi (Phillips)", func(c *canvas, x, y, s float64) {
		headRing(c, x, y, s)
		c.rect(x+.2*s, y+.5*s-.085*s, .6*s, .17*s)
		c.rect(x+.5*s-.085*s, y+.2*s, .17*s, .6*s)
	}},
	{[]string{"pz", "pozi"}, "pozidriv", func(c *canvas, x, y, s float64) {
		headRing(c, x, y, s)
		c.rect(x+.2*s, y+.5*s-.085*s, .6*s, .17*s)
		c.rect(x+.5*s-.085*s, y+.2*s, .17*s, .6*s)
		c.rotRect(x+.5*s, y+.5*s, .56*s, .07*s, 45)
		c.rotRect(x+.5*s, y+.5*s, .56*s, .07*s, 135)
	}},
	{[]string{"slot", "ura"}, "uraruuvi", func(c *canvas, x, y, s float64) {
		headRing(c, x, y, s)
		c.rect(x+.18*s, y+.5*s-.085*s, .64*s, .17*s)
	}},
	{[]string{"hex", "kuusio"}, "kuusiokolo (insex)", func(c *canvas, x, y, s float64) {
		headRing(c, x, y, s)
		c.poly(regPoly(x+.5*s, y+.5*s, .24*s, 6, 0), "f")
	}},
	{[]string{"torx"}, "torx", func(c *canvas, x, y, s float64) {
		headRing(c, x, y, s)
		c.poly(star(x+.5*s, y+.5*s, .27*s, .17*s, 6, 90), "f")
	}},
	{[]string{"nut", "mutteri"}, "mutteri", func(c *canvas, x, y, s float64) {
		c.p("%.4f w", .11*s)
		c.poly(regPoly(x+.5*s, y+.5*s, .47*s, 6, 0), "S")
		c.ring(x+.5*s, y+.5*s, .19*s, .10*s)
	}},
	{[]string{"washer", "levy"}, "aluslevy", func(c *canvas, x, y, s float64) {
		c.ring(x+.5*s, y+.5*s, .44*s, .11*s)
		c.ring(x+.5*s, y+.5*s, .19*s, .10*s)
	}},
	{[]string{"pan", "kupu"}, "kupukanta (sivukuva)", func(c *canvas, x, y, s float64) {
		c.p("%.4f %.4f m", x+.11*s, y+.55*s)
		c.p("%.4f %.4f %.4f %.4f %.4f %.4f c h f", x+.11*s, y+1.0*s, x+.92*s, y+1.0*s, x+.92*s, y+.55*s)
		c.rect(x+.4*s, y+.0*s, .2*s, .56*s)
	}},
	{[]string{"csk", "uppo"}, "uppokanta (sivukuva)", func(c *canvas, x, y, s float64) {
		c.poly([][2]float64{{x + .05*s, y + .92*s}, {x + .95*s, y + .92*s}, {x + .62*s, y + .52*s}, {x + .38*s, y + .52*s}}, "f")
		c.rect(x+.4*s, y+.0*s, .2*s, .54*s)
	}},
}

var symByName = map[string]*symbol{}

func init() {
	for i := range symbols {
		for _, n := range symbols[i].names {
			symByName[n] = &symbols[i]
		}
	}
}

// ---------- tekstin jäsennys ja asettelu ----------

type item struct {
	text string  // tekstijakso, tai tyhjä jos symboli
	sym  *symbol // symboli
}

func parseLine(s string) []item {
	var items []item
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			items = append(items, item{text: cur.String()})
			cur.Reset()
		}
	}
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] == ':' {
			for j := i + 1; j < len(rs) && j <= i+10; j++ {
				if rs[j] == ':' {
					if sy, ok := symByName[strings.ToLower(string(rs[i+1:j]))]; ok {
						flush()
						items = append(items, item{sym: sy})
						i = j
						goto next
					}
					break
				}
			}
		}
		cur.WriteRune(rs[i])
	next:
	}
	flush()
	return items
}

const (
	capH  = 0.718 // Helveticaan versaalikorkeus (em)
	extYH = 0.925 // ylä- ja alapidennysten yhteiskorkeus (em)
)

func lineWidth(items []item, fs float64, bold bool) float64 {
	w := 0.0
	for _, it := range items {
		if it.sym != nil {
			w += capH * fs
			continue
		}
		for _, r := range it.text {
			w += glyphWidth(r, bold) * fs
		}
	}
	return w
}

type options struct {
	tape, pad, vmargin, voffset, fixedLen, maxLen, minLen float64
	bold                                                  bool
	align                                                 string
	rot                                                   int
}

type page struct {
	w, h    float64 // mm
	content []byte
}

func renderLabel(text string, o options) page {
	var lines [][]item
	for _, l := range strings.Split(text, "|") {
		lines = append(lines, parseLine(strings.TrimSpace(l)))
	}
	n := float64(len(lines))
	avail := (o.tape - 2*o.vmargin) / n
	fs := avail / extYH * 0.98

	maxW := 0.0
	for _, l := range lines {
		maxW = math.Max(maxW, lineWidth(l, fs, o.bold))
	}
	limit := o.maxLen
	if o.fixedLen > 0 {
		limit = o.fixedLen
	}
	if limit > 0 && maxW > limit-2*o.pad {
		fs *= (limit - 2*o.pad) / maxW
		maxW = limit - 2*o.pad
	}
	W := math.Max(maxW+2*o.pad, o.minLen)
	if o.fixedLen > 0 {
		W = o.fixedLen
	}
	T := o.tape

	var c canvas
	pw, ph := W, T
	c.p("q")
	c.p("%.6f 0 0 %.6f 0 0 cm", ptPerMM, ptPerMM)
	switch o.rot {
	case 90:
		pw, ph = T, W
		c.p("0 1 -1 0 %.4f 0 cm", T)
	case 270:
		pw, ph = T, W
		c.p("0 -1 1 0 0 %.4f cm", W)
	}
	c.p("0 g 0 G")
	font := "/F1"
	if o.bold {
		font = "/F2"
	}
	for i, l := range lines {
		cy := T - o.vmargin - avail*(float64(i)+0.5) - o.voffset
		base := cy - (capH-0.207)/2*fs
		lw := lineWidth(l, fs, o.bold)
		x := o.pad
		switch o.align {
		case "center":
			x = o.pad + (W-2*o.pad-lw)/2
		case "right":
			x = W - o.pad - lw
		}
		for _, it := range l {
			if it.sym != nil {
				s := capH * fs
				it.sym.draw(&c, x, base+capH*fs/2-s/2, s)
				x += s
				continue
			}
			c.p("BT %s %.4f Tf %.4f %.4f Td (%s) Tj ET", font, fs, x, base, pdfString(it.text))
			for _, r := range it.text {
				x += glyphWidth(r, o.bold) * fs
			}
		}
	}
	c.p("Q")
	return page{w: pw, h: ph, content: c.b.Bytes()}
}

// ---------- PDF ----------

func buildPDF(pages []page) []byte {
	var out bytes.Buffer
	nObj := 4 + 2*len(pages)
	offsets := make([]int, nObj+1)
	out.WriteString("%PDF-1.4\n")
	obj := func(n int, body string) {
		offsets[n] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	var kids []string
	for i := range pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", 5+2*i))
	}
	obj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	obj(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))
	obj(3, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	obj(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	for i, p := range pages {
		obj(5+2*i, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.3f %.3f] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>",
			p.w*ptPerMM, p.h*ptPerMM, 6+2*i))
		obj(6+2*i, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(p.content), p.content))
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", nObj+1)
	for i := 1; i <= nObj; i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", nObj+1, xref)
	return out.Bytes()
}

// ---------- tulostus ----------

func findPrinter() (string, error) {
	out, err := exec.Command("lpstat", "-p").Output()
	if err != nil {
		return "", fmt.Errorf("lpstat epäonnistui: %w", err)
	}
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && f[0] == "printer" {
			lo := strings.ToLower(f[1])
			if strings.Contains(lo, "dymo") || strings.Contains(lo, "labelmanager") {
				return f[1], nil
			}
		}
	}
	return "", fmt.Errorf("DYMO-tulostinta ei löytynyt (lpstat -p). Anna nimi: -printer NIMI")
}

type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

func main() {
	var (
		o       options
		dry     bool
		pdfPath string
		printer string
		copies  int
		media   string
		list    bool
		regular bool
		cupsOpt listFlag
	)
	flag.Float64Var(&o.tape, "tape", 35/ptPerMM, "teipin leveys mm (ajurin 12 mm sivu = 35 pt)")
	flag.Float64Var(&o.pad, "pad", 1.5, "sivumarginaali mm (kummallakin puolella)")
	flag.Float64Var(&o.vmargin, "vmargin", 1, "ylä-/alamarginaali mm")
	flag.Float64Var(&o.voffset, "voffset", 2.4, "siirrä sisältöä alaspäin mm (negatiivinen = ylöspäin)")
	flag.Float64Var(&o.fixedLen, "len", 0, "kiinteä tarran pituus mm (0 = tekstin mittainen)")
	flag.Float64Var(&o.maxLen, "max", 42, "tarran enimmäispituus mm; pidempi teksti pienennetään")
	flag.Float64Var(&o.minLen, "min", 0, "tarran vähimmäispituus mm")
	flag.StringVar(&o.align, "align", "left", "tasaus: left | center | right (vaikuttaa vain kiinteällä -len)")
	flag.IntVar(&o.rot, "rot", 270, "sivun kierto: 270 | 90 (pystysivu, ajurin vaatima) | 0 (vaaka)")
	flag.BoolVar(&regular, "regular", false, "normaali lihavoidun sijaan")
	flag.BoolVar(&dry, "n", false, "älä tulosta, vain luo PDF (avaa Esikatselussa macOS:lla)")
	flag.StringVar(&pdfPath, "pdf", "", "PDF:n tallennuspolku (oletus: väliaikainen)")
	flag.StringVar(&printer, "printer", "", "tulostimen CUPS-nimi (oletus: etsi DYMO)")
	flag.IntVar(&copies, "c", 1, "kopioiden määrä")
	flag.StringVar(&media, "media", "auto", "CUPS media: auto (Custom.35xN pisteinä) | none | esim. w35h144")
	flag.Var(&cupsOpt, "o", "lisäasetus lp:lle (toistettava), esim. -o fit-to-page")
	flag.BoolVar(&list, "list", false, "listaa symbolit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "käyttö: tarra [valinnat] \"teksti\" [\"toinen tarra\" ...]")
		fmt.Fprintln(os.Stderr, "  | = rivinvaihto, :symboli: = symboli (katso -list). Valinnat ennen tekstiä.")
		flag.PrintDefaults()
	}
	flag.Parse()
	o.bold = !regular

	if list {
		var rows []string
		for _, s := range symbols {
			rows = append(rows, fmt.Sprintf("  :%s:  %s", strings.Join(s.names, ": / :"), s.desc))
		}
		sort.Strings(rows)
		fmt.Println(strings.Join(rows, "\n"))
		return
	}
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if o.rot != 0 && o.rot != 90 && o.rot != 270 {
		fmt.Fprintln(os.Stderr, "-rot: sallitut arvot 0, 90, 270")
		os.Exit(2)
	}

	var pages []page
	for _, t := range flag.Args() {
		pages = append(pages, renderLabel(t, o))
	}

	if dry || pdfPath != "" {
		out := pdfPath
		if out == "" {
			out = fmt.Sprintf("%s/tarra-%d.pdf", os.TempDir(), time.Now().UnixNano())
		}
		if err := os.WriteFile(out, buildPDF(pages), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "PDF:n kirjoitus epäonnistui:", err)
			os.Exit(1)
		}
		fmt.Println("PDF:", out)
		if dry {
			if runtime.GOOS == "darwin" {
				_ = exec.Command("open", out).Run()
			}
			return
		}
	}

	if printer == "" {
		p, err := findPrinter()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		printer = p
	}

	hasContinuous := false
	for _, c := range cupsOpt {
		if strings.HasPrefix(c, "DymoContinuousPaper=") {
			hasContinuous = true
		}
	}

	// Jokainen tarra on oma työnsä, koska paperikoko (pituus) on työkohtainen.
	for i, p := range pages {
		tmp := fmt.Sprintf("%s/tarra-%d-%d.pdf", os.TempDir(), time.Now().UnixNano(), i)
		if err := os.WriteFile(tmp, buildPDF([]page{p}), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "PDF:n kirjoitus epäonnistui:", err)
			os.Exit(1)
		}
		args := []string{"-d", printer, "-n", fmt.Sprint(copies)}
		m := media
		if m == "auto" {
			m = fmt.Sprintf("Custom.%dx%d", int(math.Round(p.w*ptPerMM)), int(math.Round(p.h*ptPerMM)))
		}
		if m != "" && m != "none" {
			args = append(args, "-o", "media="+m)
		}
		if !hasContinuous {
			args = append(args, "-o", "DymoContinuousPaper=1")
		}
		for _, c := range cupsOpt {
			args = append(args, "-o", c)
		}
		args = append(args, tmp)
		cmd := exec.Command("lp", args...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		err := cmd.Run()
		os.Remove(tmp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "lp epäonnistui:", err)
			os.Exit(1)
		}
	}
}
