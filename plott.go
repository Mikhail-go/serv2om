//svgplot -- plot data (a stream of x,y coordinates)
// +build !appengine

package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"
	"math"
	"os"

	"github.com/ajstarks/svgo"
)

// rawdata defines data as float64 x,y coordinates
//-type rawdata struct {
//-	x float64
//-	y float64
//-}

type options map[string]bool
type attributes map[string]string
type measures map[string]int

// plotset defines plot metadata
type plotset struct {
	opt  options
	attr attributes
	size measures
}

var (
	canvas                                                       = svg.New(os.Stdout)
	plotopt                                                      = options{}
	plotattr                                                     = attributes{}
	plotnum                                                      = measures{}
	ps                                                           = plotset{plotopt, plotattr, plotnum}
	plotw, ploth, plotc, gwidth, gheight, gutter, beginx, beginy int
)

const (
	globalfmt = "font-family:%s;font-size:%dpt;stroke-width:%dpx"
	linestyle = "fill:none;stroke:"
	linefmt   = "fill:none;stroke:%s"
	barfmt    = linefmt + ";stroke-width:%dpx"
	ticfmt    = "stroke:rgb(200,200,200);stroke-width:1px"
	labelfmt  = ticfmt + ";text-anchor:end;fill:black"
	textfmt   = "stroke:none;baseline-shift:-33.3%"
	smallint  = -(1 << 30)
)
//var lsh int//сдвиг начала (часы)
     //tlm int  ширина (минуты) для графика
var t0, t1 , wrx time.Duration      //начальное, конечное время, окно усреднения

// init initializes command flags and sets default options
func init() {

	// boolean options
	showx := flag.Bool("showx", true, "show the xaxis")
	showy := flag.Bool("showy", true, "show the yaxis")
	showbar := flag.Bool("showbar", false, "show data bars")
	area := flag.Bool("area", false, "area chart")
	connect := flag.Bool("connect", true, "connect data points")
	showdot := flag.Bool("showdot", false, "show dots")
	showbg := flag.Bool("showbg", true, "show the background color")
	showfile := flag.Bool("showfile", false, "show the filename")
	sameplot := flag.Bool("sameplot", false, "plot on the same frame")

	// attributes
	bgcolor := flag.String("bgcolor", "rgb(240,240,240)", "plot background color")
	barcolor := flag.String("barcolor", "gray", "bar color")
	dotcolor := flag.String("dotcolor", "black", "dot color")
	linecolor := flag.String("linecolor", "gray", "line color")
	areacolor := flag.String("areacolor", "gray", "area color")
	font := flag.String("font", "Calibri,sans", "font")
	labelcolor := flag.String("labelcolor", "black", "label color")
	plotlabel := flag.String("label", "", "plot label")
    spsets := flag.String("spsets", "/dev/ttyUSB0 9600 2 N", "serial port settings")

	// sizes
	dotsize := flag.Int("dotsize", 2, "dot size")
	linesize := flag.Int("linesize", 2, "line size")
	barsize := flag.Int("barsize", 2, "bar size")
	fontsize := flag.Int("fontsize", 11, "font size")
	xinterval := flag.Int("xint", 60, "x axis interval") //10
	yinterval := flag.Int("yint", 4, "y axis interval")
	ymin := flag.Int("ymin", smallint, "y minimum")
	ymax := flag.Int("ymax", smallint, "y maximum")
    divy := flag.Int("divy", 10, "y делитель")
    tlm := flag.Int("tlm", 180, "ширина интервала (минут) для графика")
    lsh := flag.Int("lsh",0, "сдвиг назад в часах ")
    tlom := flag.Int("tlom", 10, "интервал опроса ОМ310 (секунд)")
	// meta options
	flag.IntVar(&beginx, "bx", 100, "initial x")
	flag.IntVar(&beginy, "by", 50, "initial y")
	flag.IntVar(&plotw, "pw", 900, "plot width") //500
	flag.IntVar(&ploth, "ph", 500, "plot height")
	flag.IntVar(&plotc, "pc", 2, "plot columns")
	flag.IntVar(&gutter, "gutter", ploth/10, "gutter")
	flag.IntVar(&gwidth, "width", 1024, "canvas width")
	flag.IntVar(&gheight, "height", 768, "canvas height")

	flag.Parse()

	// fill in the plotset -- all options, attributes, and sizes
	plotopt["showx"] = *showx
	plotopt["showy"] = *showy
	plotopt["showbar"] = *showbar
	plotopt["area"] = *area
	plotopt["connect"] = *connect
	plotopt["showdot"] = *showdot
	plotopt["showbg"] = *showbg
	plotopt["showfile"] = *showfile
	plotopt["sameplot"] = *sameplot

	plotattr["bgcolor"] = *bgcolor
	plotattr["barcolor"] = *barcolor
	plotattr["linecolor"] = *linecolor
	plotattr["dotcolor"] = *dotcolor
	plotattr["areacolor"] = *areacolor
	plotattr["font"] = *font
	plotattr["label"] = *plotlabel
	plotattr["labelcolor"] = *labelcolor
    plotattr["spsets"] = *spsets

	plotnum["dotsize"] = *dotsize
	plotnum["linesize"] = *linesize
	plotnum["fontsize"] = *fontsize
	plotnum["xinterval"] = *xinterval
	plotnum["yinterval"] = *yinterval
	plotnum["barsize"] = *barsize
	plotnum["ymin"] = *ymin
	plotnum["ymax"] = *ymax
    plotnum["divy"] = *divy
    plotnum["tlm"] = *tlm
    plotnum["lsh"] = *lsh
    plotnum["tlom"] = *tlom
}

// fmap maps world data to document coordinates
func fmap(value float64, low1 float64, high1 float64, low2 float64, high2 float64) float64 {
	normv := 0.
	if (high1 != low1) { normv = (value-low1)/(high1-low1) }
	return low2 + (high2-low2)*normv
}
// doplot get points and makes a plot
func doplot(x, y int, i, j, k int, rdts01 []rdt, ndt int, wrx time.Duration) {
	data := getpoints(rdts01, ndt, i, j, k , wrx)
	//fmt.Println(data)
		if len(data) >= 2 {
		plot(x, y, plotw, ploth, ps, data)
		}
}

// plot places a plot at the specified location with the specified dimemsions
// usinng the specified settings, using the specified data
func plot(x, y, w, h int, settings plotset, d []point) {
	nd := len(d)
	if nd < 2 {
		fmt.Fprintf(os.Stderr, "%d is not enough points to plot\n", len(d))
		return
	}
	// Compute the minima and maxima
	minx := d[0].t.Seconds()
	maxx := d[nd-1].t.Seconds()
	miny, maxy := float64(settings.size["ymin"]), float64(settings.size["ymax"])  //d[0].d , d[0].d

	if settings.size["ymin"] != smallint {
		miny = float64(settings.size["ymin"])
	}
	if settings.size["ymax"] != smallint {
		maxy = float64(settings.size["ymax"])
	}
	// Prepare for a area or line chart by allocating
	// polygon coordinates; for the hrizon plot, you need two extra coordinates
	// for the extrema.
	needpoly := settings.opt["area"] || settings.opt["connect"]
	var xpoly, ypoly []int
	if needpoly {
		xpoly = make([]int, nd+2)
		ypoly = make([]int, nd+2)
		// preload the extrema of the polygon,
		// the bottom left and bottom right of the plot's rectangle
		xpoly[0] = x
		ypoly[0] = y + h
		xpoly[nd+1] = x + w
		ypoly[nd+1] = y + h
	}
	// Loop through the data, drawing items as specified
	spacer := 10
	canvas.Gstyle(fmt.Sprintf(globalfmt,
		settings.attr["font"], settings.size["fontsize"], settings.size["linesize"]))

	for i, v := range d {
		xp := int(fmap(v.t.Seconds(), minx, maxx, float64(x), float64(x+w)))
		yp := int(fmap(v.d, miny, maxy, float64(y), float64(y-h)))

		if needpoly {
			xpoly[i+1] = xp
			ypoly[i+1] = yp + h
		}
		if settings.opt["showbar"] {
			canvas.Line(xp, yp+h, xp, y+h,
				fmt.Sprintf(barfmt, settings.attr["barcolor"], settings.size["barsize"]))
		}
		if settings.opt["showdot"] {
			canvas.Circle(xp, yp+h, settings.size["dotsize"], "fill:"+settings.attr["dotcolor"])
		}
	}
	//++	
		if settings.opt["showx"] {
			tics := tictime(d[0].t, d[nd-1].t, settings.size["tlm"])///tlm)
            t0r := T0 //.Round(time.Minute)  
			for _ , vt := range(tics) {
			xp := int(fmap(vt.Seconds(), minx, maxx, float64(x), float64(x+w)))
			canvas.Text(xp, (y+h)+(spacer*2), fmt.Sprintf("%s", t0r.Add(vt).Format("Jan 2 15:04:05")), "text-anchor:middle")
			canvas.Line(xp, y, xp, (y+h)+spacer, ticfmt)	//
			}
		}
	//++
	//fmt.Println(ypoly) //d
	// Done constructing the points for the area or line plots, display them in one shot
	if settings.opt["area"] {
		canvas.Polygon(xpoly, ypoly, "fill:"+settings.attr["areacolor"])
	}

	if settings.opt["connect"] {
		canvas.Polyline(xpoly[1:nd+1], ypoly[1:nd+1], linestyle+settings.attr["linecolor"])
	}
	// Put on the y axis labels, if specified
	if settings.opt["showy"] {
		bot := math.Floor(miny)
		top := math.Ceil(maxy)
		yrange := top - bot
	if yrange > 0 { 
		interval := yrange / float64(settings.size["yinterval"])
		canvas.Gstyle(labelfmt)
		for yax := bot; yax <= top; yax += interval {
			yaxp := fmap(yax, bot, top, float64(y), float64(y-h))
			canvas.Text(x-spacer, int(yaxp)+h, fmt.Sprintf("%.1f", yax/float64(settings.size["divy"])), textfmt)
			canvas.Line(x-spacer, int(yaxp)+h, x+w, int(yaxp)+h) //
		}
		canvas.Gend()
	}
	}
	canvas.Gend()
}

func plotgk(x, y int, i, j int, lblnam []string, t0, t1, wrx time.Duration) {
	var ymi, yma uint16
    //fmt.Println(len(rdts)) //d
	rdts01, ndt, err := getsn(rdts, t0, t1)	//ndt количество точек в границах t0, t1
		if err != nil {
		fmt.Println(err)
		return
		}
    //fmt.Println(maxim.D[i][j][0:])	//d
    yma = maxim.D[i][j][0]
   // ymi = minim.D[i][j][0]
    for k , _ := range maxim.D[i][j][0:] {
		if yma < maxim.D[i][j][k] {yma = maxim.D[i][j][k]}
	//	if ymi > minim.D[i][j][k] {ymi = minim.D[i][j][k]}
	}
	ps.size["ymax"] = int(yma)
	ps.size["ymin"] = int(ymi) //==0
    if (i == 1) && (j ==0) { ps.size["ymin"] = 180} //минимальное напряжение для графика 
    ps.size["divy"] = diup[i][j]
	for k, lbl := range lblnam {
		if k == 0 {
		ps.attr["linecolor"] = "yellow" 
		}
		if k == 1 {ps.attr["linecolor"] = "green" }
		if k == 2 {ps.attr["linecolor"] = "red" }
	if len(lbl) == 0 {continue}
	ps.attr["label"] = lbl 
	doplot(x, y, i, j, k, rdts01, ndt, wrx)
	ps.attr["labelcolor"] = ps.attr["linecolor"]
	canvas.Text(x, y+(20 * (k+1)), ps.attr["label"], "font-size:120%;fill:"+ps.attr["labelcolor"])
	}
}

func getsn(rdts []rdt, t0, t1 time.Duration) (xrdts []rdt, ndt int, err error) {	
	var it int
	var i0x int
	var dt rdt
	// t1 время конечной точки относительно t00
	// t0 время начальной точки относительно t00
        if len(rdts) == 0 {
        err = fmt.Errorf(" Нет данныx c ОМ310")
        return nil, 0, err
        }	
		if t0 > (rdts[len(rdts)-1].rt) {
        err = fmt.Errorf(" Нет данных в заданном диапазоне")
		return nil, 0, err
		} 	
	for it, dt = range(rdts) {
			if (dt.rt < t0)  {
			i0x = it
			continue
			}
			if dt.rt < t1 {
			ndt ++
			}
	}
		if i0x > 0 {i0x++} //
		// fmt.Println("i0x=",i0x, "ndt=", ndt)
		xrdts = rdts[i0x:] //
    //clear
		for i:= 0; i < 3 ; i++ {
			for j:= 0; j < 5 ; j++ {
					for k, _ := range (maxim.D[i][j][0:]) {
						maxim.D[i][j][k] = 0
					//	maxim.T[i][j][k] = dt.rt	
					//x := minim.D[i][j][k]
					//	if x > xd {
					//	minim.D[i][j][k] = xd
					//	minim.T[i][j][k] = dt.rt
					//	}
					}
			}
		}
	//вычисляем максимумы и минимумы с привязкой ко времени 
		for _, dt = range(xrdts) {
			for i:= 0; i < 3 ; i++ {
				for j:= 0; j < 5 ; j++ {
					for k, xm := range (maxim.D[i][j][0:]) {
					xd := dt.rd[i][j][k]
						if xm < xd {
						maxim.D[i][j][k] = xd
						maxim.T[i][j][k] = dt.rt
						}
					x := minim.D[i][j][k]
						if x > xd {
						minim.D[i][j][k] = xd
						minim.T[i][j][k] = dt.rt
						}
					}
				 }
			}
		}
	return xrdts, ndt, err	//ndt количество точек в границах t0:t1
}
func getpoints(rdts []rdt, ndt, iom int, jom int, kom int, rot time.Duration) (dtps []point) {
	var dtp point
	//var dt rdt 
	// rot -ширина окна усреднения =  tlm/360
        if len(rdts) == 0 || ndt == 0  {
        return nil
        }
	dtp.t = rdts[0].rt.Round(rot)
	nx:=0
	//for i, x:= range(rdts[0:]) {
	for i:= 0; i < ndt; i++ {			//ndt количество точек в границах t0, t1
	x := rdts[i]
	rx := x.rt.Round(rot)    //
		if rx == dtp.t {
		xd := rdts[i].rd[iom][jom][kom]
		dtp.d = float64(xd) + dtp.d
		nx++
			//if (i +1)  == len(rdts[0:]) {
			if (i +1)  == ndt {
			fnx := float64(nx)
 			dtp.d = (dtp.d/fnx) //* kiup[iom][jom]
			dtps = append(dtps,dtp) //если последний dtp.t = предыдущему
			///fmt.Println(i, nx, dtp.t, dtp.d)
			}
		continue
		}
  	fnx := float64(nx)
 	dtp.d = (dtp.d/fnx) //* kiup[iom][jom]  
	dtps = append(dtps, dtp)		//предыдущий dt сохраняем
	//fmt.Println(i-1, nx, dtp.t, dtp.d)
	dtp.t = rx			//текущий получаем
	dtp.d = float64(rdts[i].rd[iom][jom][kom])
	nx = 1
		//if (i +1)  == len(rdts[0:]) {
		if (i +1)  == ndt { 
		dtp.d = dtp.d //* kiup[iom][jom]  //если последний
		dtps = append(dtps,dtp)  //сохраняем
	//	fmt.Println(i, nx, dtp.t, dtp.d)
		}	
	}
return dtps
}
func tictime(tmin, tmax time.Duration, tl int) []time.Duration {
	ntic := 6 // число интервалов просмотра
	ltic := tl * 60 / ntic      // длина интервала в секундах
	if ltic == 0 {return nil}
	dltic := time.Duration(ltic) * time.Second
	xmin := tmin.Round(dltic)
	if xmin < tmin { xmin += dltic } 
	data := make([]time.Duration, 1)
	data[0] = xmin
    x := xmin
		for i := 0 ; x <= tmax; i++ {
			if i > 0 {
			data = append(data, x)
			}	
		x += dltic
		}
	return data[0:]
}

// сервис http://host:8181/if3
func plotif3(w http.ResponseWriter, req *http.Request) {
    var lblname = []string {"Ток фаза А","Ток фаза B","Ток фаза C"}
	canvas = svg.New(w)
  w.Header().Set("Content-Type", "image/svg+xml")
	canvas.Start(gwidth, gheight)
	canvas.Rect(0, 0, gwidth, gheight, "fill:white")
    // Draw the plot's bounding rectangle
	if ps.opt["showbg"] && !ps.opt["sameplot"] {
		canvas.Rect(beginx, beginy, plotw, ploth, "fill:"+ps.attr["bgcolor"])
	}
    tn := time.Now().Sub(T0)	//.Round(1 * time.Second)
    tlmx := ps.size["tlm"]
	t1 = tn - time.Duration(ps.size["lsh"]) * time.Hour //сдвиг конечной точки (относительно T0)
	t0 = t1 - (time.Duration(tlmx) * time.Minute )//время начальной точки
	wrx = (time.Duration(tlmx) * time.Minute/360).Round(1* time.Millisecond) //окно усреднения (миллисекунды)
	plotgk(beginx, beginy, 0, 0, lblname, t0, t1, wrx)
	canvas.End() 		
}
// сервис http://host:8181/uf3
func plotuf3(w http.ResponseWriter, req *http.Request) {
    var lblname = []string {"U(в) фаза А","U(в) фаза B","U(в) фаза C"}
	canvas = svg.New(w)
  w.Header().Set("Content-Type", "image/svg+xml")
	canvas.Start(gwidth, gheight)
	canvas.Rect(0, 0, gwidth, gheight, "fill:white")
    // Draw the plot's bounding rectangle
	if ps.opt["showbg"] && !ps.opt["sameplot"] {
		canvas.Rect(beginx, beginy, plotw, ploth, "fill:"+ps.attr["bgcolor"])
	}
    tn := time.Now().Sub(T0)	//.Round(1 * time.Second)
    tlmx := ps.size["tlm"]
	t1 = tn - time.Duration(ps.size["lsh"]) * time.Hour //сдвиг конечной точки (относительно T0)
	t0 = t1 - (time.Duration(tlmx) * time.Minute )//время начальной точки
	wrx = (time.Duration(tlmx) * time.Minute/360).Round(1* time.Millisecond) //окно усреднения (миллисекунды)
	plotgk(beginx, beginy, 1, 0, lblname, t0, t1, wrx)
	canvas.End() 		
}


