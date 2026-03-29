package main

import (
	"fmt"
	"net"
	"os"
	//"github.com/tarm/serial"
	"time"
	"log"
	"net/http"
	"bytes"
    "strings"
    "strconv"
	"github.com/Mikhail-go/mbus"
)
    var ticker *time.Ticker //таймер опроса ОМ310
	var T0, t01 time.Time //начальное время
    var Tsm string // оно же в тексте
    var Rpwh0, rpwh0 []float64 //начальное потребление по фазам квт*час
	var Tpwh0, tpwh0 float64 //суммарное начальное потребление квт*час
    var Np int // общая статистика
    var polrun uint16 // флаг включения монитоинга
    type dat [3][5][3]uint16
	type rdt struct {
	rd dat
	rt time.Duration
	}
    type point struct {
	d float64
	t time.Duration //time.Duration (интервал времени)
	}
    var diup = [][]int {{10, 10, 10, 10, 10},	//делитель для токов (a)
				  {1, 1, 1, 1, 1},		//напряжений (в)
				  {100, 100, 100, 100, 1000}} //мощностей (квт/ква), косинус фи (1000)
	//var lblname = []string {"Ток фаза А","Ток фаза B","Ток фаза C"}
	var data []point
	var rdts []rdt  //сюда копятся данные с ОМ310
    var dt rdt      //текущие данные с ома с привязкой по времени
    type dattim struct {	//тип данных макс./мин. за всё время мониторинга
		D [3][5][]uint16 //данные IUP ([3][5][] ток, напряжение, мощность)
		T [3][5][]time.Duration //моменты времени их регистрации
    }
    var Max = dattim {[3][5][]uint16{{{0,0,0}, {0,0,0}, {0,0,0}, {0}, {0}}, // максимумы за всё время 
			  {{0,0,0}, {0,0,0}, {0}, {0}, {0}}, //U
			  {{0,0,0}, {0}, {0}, {0}, {0,0,0}}}, //P
			[3][5][]time.Duration {{{0,0,0}, {0,0,0}, {0,0,0}, {0}, {0}}, //
			 {{0,0,0}, {0,0,0}, {0}, {0}, {0}},                 //
			 {{0,0,0}, {0}, {0}, {0}, {0, 0, 0}}}} //     
    var maxim = dattim {[3][5][]uint16{{{0,0,0}, {0,0,0}, {0,0,0}, {0}, {0}}, // максимумы по интервалу просмотра
			  {{0,0,0}, {0,0,0}, {0}, {0}, {0}}, //U
			  {{0,0,0}, {0}, {0}, {0}, {0,0,0}}}, //P
			[3][5][]time.Duration {{{0,0,0}, {0,0,0}, {0,0,0}, {0}, {0}}, //
			 {{0,0,0}, {0,0,0}, {0}, {0}, {0}},                 //
			 {{0,0,0}, {0}, {0}, {0}, {0, 0, 0}}}} //     
	var Min = dattim {[3][5][]uint16{{{65000,65000,65000}, {65000,65000,65000}, {65000,65000,65000}, {65000}, {65000}}, // и минимумы
			  {{65000,65000,65000}, {65000,65000,65000}, {65000}, {65000}, {65000}},
			  {{65000,65000,65000}, {65000}, {65000}, {65000}, {65000,65000,65000}}},
			[3][5][]time.Duration {{{0,0,0}, {0,0,0}, {0,0,0}, {0}, {0}},
			 {{0,0,0}, {0,0,0}, {0}, {0}, {0}},
			 {{0,0,0}, {0}, {0}, {0}, {0, 0, 0}}}}
    var minim = dattim {[3][5][]uint16{{{65000,65000,65000}, {65000,65000,65000}, {65000,65000,65000}, {65000}, {65000}}, //соотверственно
			  {{65000,65000,65000}, {65000,65000,65000}, {65000}, {65000}, {65000}},
			  {{65000,65000,65000}, {65000}, {65000}, {65000}, {65000,65000,65000}}},
			[3][5][]time.Duration {{{0,0,0}, {0,0,0}, {0,0,0}, {0}, {0}},
			 {{0,0,0}, {0,0,0}, {0}, {0}, {0}},
			 {{0,0,0}, {0}, {0}, {0}, {0, 0, 0}}}} 
    var namr = [][]string {{"Токи фаз", "Cредние значения за минуту", "Пиковые значения ",
			"Нулевая последовательность", "Обратная последовательность"},
			{"Напряжения фаз", "Линейные напряжения", "Прямая последовательность", 
			"Обратная последовательность", "Нулевая последовательность" },
			{"Активная по фазам", "Суммарная активная", "Реактивная",
			"Полная", "Косинус фи по фазам"}}
func httphandler(w http.ResponseWriter, r *http.Request) {
	var er error
	var bufs bytes.Buffer //буфер выходной строки для передачи ответа клиенту
    var lenj int //длина среза j-ой компоненты 
//-- Измеряемые величины ОМ310
	//var iF, iS, iN, iF0, ioP om.Dfom  // фазные токи, средние, максимальные (по фазам), нулевой и обратной последовательности 
	//var UF, UL, UpP, UoP, UnP om.Dfom // фазные и линейные напряжения, прямая, обратная и нулевая поледоватеоьность
	//var Pot, PoA, PoJ, PA, PC om.Dfom // мощности полная, активная, реактивная, активная по фазам, косинус фи по фазам
	var frqd [1]uint16 //частота тока
	var twrk mbus.Dfom //время работы Ом310
	var rss, rav mbus.Dfom //регистры состояния (240), rav (241) ОМ310
	var stload, stfunc string // состояние реле нагрузки и функционального реле
	stload = "ON"   //состояние реле нагрузи
    stfunc = stload // и функционального реле
	er = rss.Getrtu(240, 1, 1)
	if er != nil {
		fmt.Println(er)
		os.Exit(1)
	}
	switch {
	case (rss[0] & 2) == 2 :
			fmt.Println(time.Now().Format("2006 Jan 2 15:04:05"), "Реле нагрузки уже включено")
            if (rss[0] & 4) == 0 { stfunc = "OFF"} 
	case (rss[0] & 8) == 8 :
		stload = "OFF-->АПВ" //ожидается АПВ
        stfunc = stload
		fmt.Println("АПВ ...(max 5 min)")	
	case (rss[0] & 1) == 1 :
		stload = "OFF"
        stfunc = stload
		er = rav.Getrtu(241, 1, 1)
		if (rav[0] & 4) == 4 {	//отключение по максимальному току 
			er = mbus.Putrtu (220, 1, 1) //запись 1 - включение нагрузки
			fmt.Println("Выдана команда на включения нагрузки ...3 sec")
			time.Sleep(time.Millisecond * 3000) //3 сек на включение реле нагрузки
			er = mbus.Putrtu(169, 0, 1) // запрет защиты по току !
			fmt.Println("i^r 1->0 ... 3 sec") 
			time.Sleep(time.Millisecond * 3000)
			er = mbus.Putrtu(169, 1, 1) // опять разрешаем защиту по току !
			fmt.Println("i^r 0->1 ... 3 sec")
			time.Sleep(time.Millisecond * 3000)
			er = rss.Getrtu (240, 1, 1)
			    if (rss[0] & 2) == 2 {
			    fmt.Println (time.Now().Format("2006 Jan 2 15:04:05"), "Нагрузка включена")
			    stload += "--> ON" 
                stfunc = stload
			    }
		}
	}
    nrdt := len(rdts)
        if nrdt ==0 {
            bufs.WriteString(fmt.Sprintf("%s", "Нет данных !")) //  
    	fmt.Fprint (w, bufs.String()) // в ответ клиенту
        return
        }
    xdt := rdts[nrdt-1] //последняя порция из ома
  	//er = frq.Getrtu(138, 1, 1) // частота тока
    frqd[0] = xdt.rd[1][4][1] //частота тока
    rfrq := rint16(frqd[0:], 0.1)
    pwhom := make([]uint16,6)
    pwhom[0] , pwhom[1] = xdt.rd[2][1][1], xdt.rd[2][1][2] //фаза А
    pwhom[2] , pwhom[3] = xdt.rd[2][2][1], xdt.rd[2][2][2] //фаза В
    pwhom[4] , pwhom[5] = xdt.rd[2][3][1], xdt.rd[2][3][2] //фаза с
    rpwh := rint32(pwhom, 0.1, 6) //текущее потребление
    Rpwh1 := subf(rpwh,Rpwh0)  //потребление c момента старта
    rpwh1 := subf(rpwh,rpwh0)  //потребление с момента последнего обращения
    copf(rpwh, rpwh0)  //обновление rpwh0
	er = twrk.Getrtu(191, 1, 1) // время работы ОМ с момента включения (часы)
	// HTML разметка
	bufs.WriteString (`<table border="1">
	<tr><th colspan="1"> Токи (амперы)</th><th colspan="1"> Текущие значения</th><th colspan="1"> Максимумы</th><th colspan="1"> Время максимумов</th>
			<th colspan="1"> Минимумы</th><th colspan="1"> Время минимумов</th></tr>`)
	for j :=0; j <5; j++ {
    lenj = len(Max.D[0][j])
	bufs.WriteString (hformatt(0, namr[0][j], vfs(0,j,xdt.rd[0][j][:lenj]), vfs(0,j,Max.D[0][j]), bsdm(Max.T[0][j]), vfs(0,j,Min.D[0][j]), bsdm(Min.T[0][j])))
	}
	bufs.WriteString (`</table>`)
	bufs.WriteString (`<table border="1">
	<tr><th colspan="1"> Напряжения (вольты)</th><th colspan="1"> Текущие значения</th><th colspan="1"> Максимумы</th><th colspan="1"> Время максимумов</th>
			<th colspan="1"> Минимумы</th><th colspan="1"> Время минимумов</th></tr>`)
	for j :=0; j <5; j++ {
    lenj = len(Max.D[1][j])
	bufs.WriteString (hformatt(1, namr[1][j], vfs(1,j,xdt.rd[1][j][:lenj]), vfs(1,j,Max.D[1][j]), bsdm(Max.T[1][j]), vfs(1,j,Min.D[1][j]), bsdm(Min.T[1][j])))
	}
	bufs.WriteString (`</table>`)
	//-
	bufs.WriteString (`<table border="1">
	<tr><th colspan="1"> Мощности (квт/ква)</th><th colspan="1"> Текущие значения</th><th colspan="1"> Максимумы</th><th colspan="1"> Время максимумов</th>
			<th colspan="1"> Минимумы</th><th colspan="1"> Время минимумов</th></tr>`)
	for j :=0; j <5; j++ {
    lenj = len(Max.D[2][j])
	bufs.WriteString (hformatt(2, namr[2][j], vfs(2,j,xdt.rd[2][j][:lenj]), vfs(2,j,Max.D[2][j]), bsdm(Max.T[2][j]), vfs(2,j,Min.D[2][j]), bsdm(Min.T[2][j])))
	}
	bufs.WriteString (`</table>`)
	//-
	bufs.WriteString (`<table border="1">
	<tr><th colspan="1"> Потребление электроэнергии (квт*час)</th><th colspan="1"> По фазам</th><th colspan="1"> Суммарное</th>
			<th colspan="1"> За время</th></tr>`)
	bufs.WriteString (hformatp("С момента старта мониторинга", Rpwh1, sumf(Rpwh1), fmt.Sprintf("%v",time.Now().Sub(T0).Round(time.Second))))
	bufs.WriteString (hformatp("С момента последнего обращения", rpwh1, sumf(rpwh1), tnw()))
	bufs.WriteString (`</table>`)
    bufs.WriteString (`<table border="1">
	<tr><th colspan="1"> Статусы и др.</th><th colspan="1"> Реле нагрузки</th><th colspan="1"> Функциональное реле</th>
			<th colspan="1"> Частота тока (гц)</th><th colspan="1">Время работы ОМ310(часы)</th></tr>`)
    bufs.WriteString (hformat(" Состояния реле и др.", stload, stfunc, rfrq, twrk))
    bufs.WriteString (`</table>`)
	// конец разметки
    bufs.WriteString(fmt.Sprintf("%s", Sprintvola())) //  
	fmt.Fprint (w, bufs.String()) // в ответ клиенту
}
func main() {
	var pwh0 mbus.Dfom
    var er error
    var boad int
    var nstop []byte
    var parity []byte
	http.HandleFunc("/v", httphandler)
    http.HandleFunc("/if3", plotif3)
    http.HandleFunc("/uf3", plotuf3)
    //-получение настроек serial port
    spar := strings.Fields(ps.attr["spsets"])
    boad, er = strconv.Atoi(spar[1])
    nstop = []byte(spar[2])
    parity = []byte(spar[3])
    fmt.Printf("port [%s], boad [%d] nstop [%c], parity [%c]\n", spar[0], boad, nstop[0], parity[0])
    //-
    //er = mbus.Cserial("/dev/ttyUSB0", 9600, 2, 'N')
    er = mbus.Cserial(spar[0], boad, nstop[0], parity[0])
    if er != nil {	
		log.Fatal(er)
    fmt.Println (" Возможно у пользователя нет прав на /dev/ttyUSB, попробуйте запуск через sudo")
	}
    defer mbus.S.Close()
    //ipport := ps.attr["ipaddr"] + ":4545"
    ipport := ":4545"
    //hipport := ps.attr["ipaddr"] + ":8181"
    hipport := ":8181"
	listener, err := net.Listen("tcp", ipport) 
     	hlistener, err := net.Listen("tcp", hipport)
	if err != nil {
        	log.Println("error starting net.Listen: ", err) 
        return
    	} 
    	defer listener.Close()
	defer hlistener.Close()
    	fmt.Printf("Server is listening : %s \n", ipport)
	go http.Serve(hlistener, nil)
		fmt.Printf ("and http Server started on %s \n", hipport)
    T0 = time.Now() //фиксируем начальное время
    Tsm = T0.Format("2006-01-02 15:04:05") //момент времени до секунд
	//---- синхронизатор обращений к ОМ310
	go func() {
	for {
		<- mbus.Omact
	time.Sleep(time.Millisecond * 60) //ограничитель частоты обращения к ОМ310 //200
	}
	}()
	//---
    //--тик на "tlom" секунд
    polrun = 1  //сразу включаем
	ticker = time.NewTicker(time.Second * time.Duration(ps.size["tlom"]))  // создание канала таймера тиков и запуск тикера
	defer ticker.Stop()
	    //------горутина  чтения данных из Ом310 по тикам таймера 
    go func() {
  	    for {
		    select {
		    //ожидание  тика 
		        case  <-ticker.C:        
                    if polrun == 0 { continue } //        
                er := tickom()  //читаем данные из Ома с привязкой ко времени
                if er != nil {
                continue        // os.Exit(1)    
            }
            Np++  //общее количество измеренных точек i,u,p
	    }
    }
    }()
    //---
    er = pwh0.Getrtu(90, 6, 1) // чтение текущих значений электроэнергии по фазам
            if er != nil {
            fmt.Println("Ошибка при работе с ОМ310:", er)
            return
        }
    t01 = T0     //изменяемая копия
    //T0 = T0.Round(time.Minute)//?
	rpwh0 = rint32(pwh0, 0.1, 6)
    Rpwh0 = rint32(pwh0, 0.1, 6)
	Tpwh0 = rpwh0[0] + rpwh0[1] + rpwh0[2]
    tpwh0 = Tpwh0 //изменяемая копия
	//--- ожидание запроса по TCP и установка соединения
	for {         	
		conn, err := listener.Accept() 
        		if err != nil { 
        	    	log.Println(" error listener.Accept: ", err) 
        	    	conn.Close()
			break
			}        
	handleConnection(conn)  // запускаем  для обработки запроса
    	} 
}
// обработка подключения
func handleConnection(conn net.Conn) { 
	var m int
    var cs1, us16 uint16
    defer conn.Close()
    cb := make([]byte, 128 ) //
    for {
        // считываем полученные в запросе данные
        n, err := conn.Read(cb) 
        if n == 0 || err != nil {
            fmt.Println("Read error from conn:", err) 
            break
        }    
        if cb[0] == 1  {       
	        m, err = mbus.Getoms (cb) //посылаем устройству команду и получаем ответ от него
	            if m==0 || err != nil {
		        fmt.Println("Ошибка обмена с устройством:", err)
		        //break //?!
        	    }
        // отправляем ответ клиенту
        conn.Write(cb[n:m+n])
        continue
        }
        //здесь cb[0] == 0 (виртуальный modbus)
        if cb[0] == 0  {
            hb := cb[4]
            lb := cb[5]
            dru16 := uint16(hb) <<8 + uint16(lb) 
            cbb := cb[8:]
            cbb[0] = cb[0]
            cbb[1] = cb[1]      
            if cb[1] == 6  {
                if cb[3] == 2 {polrun = dru16}      //приняли от клиента новый флаг опроса ОМ310
                if cb[3] == 3 {ps.size["tlm"] = int(dru16)} //приняли новый интервал просмота ( для svg графика)
                if cb[3] == 4 {ps.size["hls"] = int(dru16)} //приняли новый сдвиг просмотра
            }
            if cb[1] != 3 {
            conn.Write(cb[:8])      //отправляем назад клиенту саму команду (т.к. не нужно обращаться к устройству)
            continue
            }
            cbb[2] = 2
            switch  { 
                case cb[3] == 0 :   //98 байт, на dt.rd (90 байт) +dt.rt (8 байт)
                    cbb[2] = 98
                    getdt(cbb[3:], 98) //получаем данные последней точки
                    cs1 = mbus.Crcsum(cbb[0:],98+3)
	                cbb[101] = byte (cs1)
	                cbb[102] = byte (cs1>>8)
                    conn.Write(cbb[:103])   //отправляем клиенту
                break
                case cb[3] == 1 : // чтение 8 байт (time.Duration) по виртуальному адресу 2: cb[1]==0 cb[2]==8
                    cbb[2] = 8            
                    getdt(cbb[3:], 8)  
                    fmt.Println(cbb[:11])
                    cs1 = mbus.Crcsum(cbb[0:],8+3)
	                cbb[11] = byte (cs1)
	                cbb[12] = byte (cs1>>8)
                    conn.Write(cbb[:13]) //возвращаем 13 байт вместе с КС
                break            
                case cb[3] > 1 :
                    if cb[3] == 2 {us16 = polrun}
                    if cb[3] == 3 {us16 = uint16(ps.size["tlm"])}
                    if cb[3] == 4 {us16 = uint16(ps.size["hls"])}
                    if cb[3] == 5 {us16 = uint16(ps.size["tlom"])} 
                    cbb[3] = byte(us16 >> 8)
                    cbb[4] = byte(us16)
                    cs1 = mbus.Crcsum(cbb[0:],2+3)
                    cbb[5] = byte(cs1)
                    cbb[6] = byte(cs1 >> 8)
                    conn.Write(cbb[:7]) //возвращаем 7 байт вместе с КС
                break
            }
            
        } 
    }
}
// сохранение времени (Tsm) или последней точки dt в срезе байтов )
func getdt(b []byte, nb int) {
    ib := 0
    var bb [24]byte
    switch nb > 7 {
        case nb == 98 :
            dtx := rdts[len(rdts)-1] 
            for i:= 0; i < 3; i++ {
                for j:=0; j <5; j++ {
                    for k:=0; k <3 ; k++ {
                    x := dtx.rd[i][j][k]
                    b[ib] = byte(x >>8) //старший байт
                    b[ib+1] = byte(x)   // и младший
                    ib++
                    ib++            
                     }
                }
            }
            u642b(dtx.rt, b[ib:])  // интервал в байты и в буфер
        case nb == 8 : 
                tn := time.Now().Sub(T0)
                u642b(tn, b[ib:])
        case nb == 20 :
                for i, xb := range(Tsm) {
                    bb[i] = byte(xb)
                 }
                for ib = 0; ib < 20; ib +=2 {
                    b[ib] = bb[ib+1]    // стартовое время загружаем в буфер старшим байтом вперёд!
                    b[ib+1] = bb[ib]
                }
    }
   return            
}
func rint16 (ix []uint16, fuct float64) (r []float64) {
	for _, x:= range ix {
	r = append (r, fuct*float64(x))
	}
	return r
}
func rint32 (ix []uint16, fuct float64, n int) (r []float64) {
	for i := 0; i < n ; i++ { 
		x := int32(ix[i+1])
		x <<= 16
		x |= int32(ix[i])
		i++
		r = append (r, fuct*float64(x))
	}
	return r
}
func hformat(s string, sload string, sfunc string , rfrq []float64, twrk mbus.Dfom) string {
     str := "<tr><td>%s</td>"
		str += "<td>%s</<td>"
        str += "<td>%s</<td>"
		str += "<td>%.0f</td>"
		str += "<td>%d</<td>"
		str += "</tr>"
	return fmt.Sprintf(str, s, sload, sfunc, rfrq, twrk)
}
func hformatt(i int, s string, x []float64, mad []float64, mat string, mid []float64, mit string ) string {
	strfmt := "<tr><td>%s</td>"
    strf := "<td>%.1f</<td>"
    if i == 1 {strf = "<td>%.0f</<td>"}
		strfmt += strf
        strfmt += strf
		strfmt += "<td>%s</td>"
		strfmt += strf
		strfmt += "<td>%s</td>"
		strfmt += "</tr>"
	return fmt.Sprintf(strfmt, s, x, mad, mat, mid, mit)
}
func hformatp(s string, x []float64, sx float64, st string ) string {
	str := "<tr><td>%s</td>"
		str += "<td>%.1f</<td>"
        str += "<td>%.1f</<td>"
		str += "<td>%s</td>"
		str += "</tr>"
	return fmt.Sprintf(str, s, x, sx, st)
}
func tnw() string {
	t1 := time.Now()
	selp := fmt.Sprintf("%v", t1.Sub(t01).Round(time.Second)) 
	t01 = t1
	return selp
}
// получение измеряемых значений из ОМ310 с привязкой к интервалу времени dt.rt, фиксация min/max
func  tickom() error { 
//-- Измеряемые величины ОМ310
	var iF, iS, iN, iF0, ioP mbus.Dfom  // фазные токи, средние, максимальные (по фазам), нулевой и обратной последовательности 
	var UF, UL, UpP, UoP, UnP mbus.Dfom // фазные и линейные напряжения, прямая, обратная и нулевая поледоватеоьность
	var Pot, PoA, PoJ, PA, PC mbus.Dfom // мощности полная, активная, реактивная, активная по фазам, косинус фи по фазам
    var frq, pwh mbus.Dfom    //частота тока и энергопотребление
   // var dt rdt                          //текущие данные с ома с привязкой ко времени
	er := iF.Getrtua(100, 3, 1) // читаем токи по фазам
	if er != nil {
		fmt.Println(er)
		//os.Exit(1)
    return er
	}
    tnt := time.Now()
    //fmt.Println(tnt.Format("2006-01-02 15:04:05")) //deb
    dt.rt = tnt.Sub(T0) //фиксируем время
	drmaxmin(0,0,iF) // сохраняем в dt.rd на min/max проверяем
	//---- перевод в нормальные единицы  и чтение остальных измерений  
	er = iS.Getrtua(104, 3, 1) //средний ток по фазам
	drmaxmin(0, 1, iS)
	er = iN.Getrtua(107, 3, 1) //максимальный ток по фазам
	drmaxmin(0, 2, iN)
	er = iF0.Getrtua(103, 1, 1) // ток нулевой последовательности
	drmaxmin(0 , 3, iF0)
	er = ioP.Getrtua(110, 1, 1) // ток обратной поледовательности (перекос)
	drmaxmin(0, 4, ioP)
	er = UF.Getrtua(111, 3, 1) // фазные напряжения
	drmaxmin(1, 0, UF)
	er = UL.Getrtua(114, 3, 1) // линейные
	drmaxmin(1, 1, UL)
	er = UpP.Getrtua(117, 1, 1) // прямая последовательность
	drmaxmin(1, 2, UpP) 
	er = UoP.Getrtua(118, 1, 1) // обратная последовательность (перекос)
	drmaxmin(1, 3, UoP)
	er = UnP.Getrtua(119, 1, 1) // нулевая последовательность
	drmaxmin(1, 4, UnP)
	er = PA.Getrtua(126, 6, 1) // активная мощность по фазам
	drmaxmin(2, 0, u21(PA)) //убираем 2-е uint16 !!! (там всегда 0)
	er = PoA.Getrtua(122, 2, 1)  // полная активная мощность
	drmaxmin(2, 1, u21(PoA)) 
	er = PoJ.Getrtua(124, 2, 1) // полная реактивная мощность
	drmaxmin(2, 2, u21(PoJ))
	er = Pot.Getrtua(120, 2, 1) // полная  мощность
	drmaxmin(2, 3, u21(Pot))  
	er = PC.Getrtua(132, 3, 1) // косинус фи по фазам
	drmaxmin(2, 4, PC)
    er = frq.Getrtua(138,1, 1) 
    dt.rd[1][4][1] = frq[0] //частота тока в свободное место dt.rd
    er = pwh.Getrtua(90,6, 1) //электроэнергия по фазам
    //--- сохраняем в сыром виде (по 2 uint16) в свободные места dt.rd
    dt.rd[2][1][1], dt.rd[2][1][2] = pwh[0], pwh[1]  //фаза А
    dt.rd[2][2][1], dt.rd[2][2][2] = pwh[2], pwh[3] //фаза В
    dt.rd[2][3][1], dt.rd[2][3][2]= pwh[4], pwh[5] //фаза с
    //-----
    rdts = append(rdts,dt)  //копим с срезе 
    return nil
}
// Формирование  значений макc./мин. в момент времени Tnow
func drmaxmin(i int, j int, r mbus.Dfom) {
	for k, x := range r {
    dt.rd[i][j][k] = x 
	if x > Max.D[i][j][k] {
		Max.D[i][j][k] = x
		Max.T[i][j][k] = dt.rt
		}
	if x < Min.D[i][j][k] {
		Min.D[i][j][k] = x
		Min.T[i][j][k] = dt.rt
		}
  	}
}
// Преобразование сырых значений 2uint16 в 1uint (только для мощностей) 
func u21(ix []uint16) (x16 []uint16) {
	for i, x := range ix {
	if (i & 1) == 1 { continue} // там нули - отбрасываем
	x16 = append(x16, x)
	}
	return x16
}
// получение физических величин токов (i=0), напряжений (i=1), мощностей (i=2) из срезов сырых данных
func vfs(i int, j int, xs []uint16) []float64 {
	var rs []float64
	for _, x := range(xs) {
	//r := Valf(i, j, k, xs)
    r := float64(x)/float64(diup[i][j])
	rs = append(rs, r)
	}
	return rs 
}
// получение абсолютного времени для интервала t (в текстовом виде)
func bsdm (t []time.Duration) string {
	var s bytes.Buffer
    var ts string
	s.WriteString("[")
	for i, x := range(t) {
        if i < (len(t)-1) { 
        ts = T0.Add(x).Format("Jan 2 15:04| ")
        }else{
             ts = T0.Add(x).Format("Jan 2 15:04") //чтобы смотрелось лучше
              }    
	sx := fmt.Sprintf("%s",ts)
	s.WriteString(sx)
	}
	s.WriteString("]")
	return s.String()
}
func Sprintvola () string {
    tse := time.Now().Format("2006 Jan 2 15:04") //текущее время (до минут)
	stat := fmt.Sprintf (" Cтарт мониторинга [%s], снято точек [%d], текущее время [%s]", Tsm, Np, tse)
	return stat
}
// разность по 2 срезам в 1 срез  
func subf(r, r0 []float64) []float64 {
    rx := make([]float64,3)
    for  i, x := range r {
    rx[i] = x - r0[i]
    }
    return rx
}
// сумма по срезу в одно значение
func sumf(r []float64) float64 {
    rx := 0.
    for  _, x := range r {
    rx += x
    }
    return rx
}
// копирование одного среза в другой
func copf(r, r0 []float64) {
    for  i, x := range r {
    r0[i] = x
    }
}
// преобразование интервала времени (uint64) срез из 8 байтов
func u642b(t time.Duration, bts []byte) {
	u32 := uint32(t) 
	hlbyte(uint16(u32), bts[0:])
	hlbyte(uint16(u32 >>16), bts[2:])
	u32h := uint32(t >>32)
	hlbyte(uint16(u32h), bts[4:])
	hlbyte(uint16(u32h >>16), bts[6:])
}
// преобразование uint16 в 2 байта (старший байт первый )
func hlbyte (ux uint16, b2 []byte) {
	uibh := ((255 <<8) & ux) >>8
	b2[0] = byte(uibh)
	b2[1] = byte(ux)
}
