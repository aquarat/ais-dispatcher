package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/tarm/serial"
)

func main() {
	loop()

	os.Exit(0)
}

func loop() {
	log.Println("aquarat's AIS Dispatcher Started")

	comPortID := flag.String("serialport",
		"/dev/ttyUSB0",
		"The serial port's path.",
	)

	comPortBaud := flag.Int("baud",
		38400,
		"The port baud.",
	)

	hostID := flag.String("host",
		"5.9.207.224",
		"Target host (IP or Domain Name).",
	)

	hostPort := flag.String("port",
		"7018",
		"The target UDP port.",
	)

	dbFile := flag.String("dbFile",
		"/dev/shm/ais.db",
		"The target SQLite DB file.",
	)

	dbUse := flag.Bool("useDB",
		false,
		"Should we write to the database ?")

	flag.Parse()

	dbChan := make(chan *[]byte, 1000)
	go initDB(*dbFile, dbChan, dbUse)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	c := &serial.Config{Name: *comPortID,
		Baud: *comPortBaud,
	}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
		debug.PrintStack()
	}
	defer s.Close()

	conn, err := net.Dial("udp", *hostID+":"+*hostPort)
	if err != nil {
		log.Println(err)
		debug.PrintStack()
		return
	}

	dieChan := make(chan bool)
	sendChan := make(chan *[]byte, 1000)
	go dispatch(sendChan, conn)

	defer conn.Close()
	defer close(sendChan)
	defer close(dieChan)
	defer close(dbChan)

	go receiver(sendChan, dbChan, s, dieChan)

	for range sigs {
		break
	}

	dieChan <- true
	close(dieChan)
}

type DBPacket struct {
	gorm.Model
	Payload string
}

func initDB(dbFile string, dbChan chan *[]byte, dbUse *bool) {
	if *dbUse {
		db, err := gorm.Open("sqlite3", dbFile)
		defer db.Close()

		if err != nil {
			log.Fatalln(err)
			debug.PrintStack()
		}

		db.CreateTable(DBPacket{})
		db.AutoMigrate(DBPacket{})

		for p := range dbChan {
			obj := DBPacket{Payload: string(*p)}
			db.Create(&obj)
		}
	} else {
		for range dbChan {
		}
	}
}

func receiver(sendChan, dbChan chan *[]byte, s *serial.Port, dieChan chan bool) {
	log.Println("receiver started")
	bReader := bufio.NewReader(s)

	for {
		select {
		case <-dieChan:
			return
		default:
			buf, err := bReader.ReadBytes('\n')
			if err != nil {
				log.Println(err)
				debug.PrintStack()
				continue
			}

			buf = []byte(strings.TrimSpace(string(buf)))

			if len(buf) < 10 {
				continue
			}

			//$GPGGA,092750.000,5321.6802,N,00630.3372,W,1,8,1.03,61.7,M,55.2,M,,*76
			if !isChecksumGood(buf) {
				continue
			}

			bufPointer := &buf
			sendChan <- bufPointer
			dbChan <- bufPointer
		}
	}
}

func recovery() {
	if err := recover(); err != nil {
		log.Println("ERROR :", err)
		debug.PrintStack()
	}
}

func isChecksumGood(sentence []byte) (crcGood bool) {
	{
		defer recovery()

		var (
			origCRC string
			crc     byte = 0
		)

		indexCRC := strings.Index(string(sentence), "*") + 1

		if len(sentence) < indexCRC || len(sentence) < 2 {
			return
		}

		for _, j := range sentence[1 : indexCRC-1] {
			crc ^= j
		}

		origCRC = string(sentence[indexCRC:])

		if strings.TrimSpace(strings.ToUpper(hex.EncodeToString([]byte{crc}))) == strings.TrimSpace(strings.ToUpper(origCRC)) {
			crcGood = true
		}
	}

	return
}

func dispatch(a chan *[]byte, conn net.Conn) {
	for p := range a {
		_, err := conn.Write(*p)
		CE(err)
		log.Println("Tx :", string(*p))
		break
	}
}

func CE(e error) {
	if e != nil {
		log.Println(e)
		debug.PrintStack()
	}
}

/*!AIVDM,1,1,,A,344Nv<5P001DG;=dVBlTs76v0P00,0*6C
!AIVDM,1,1,,B,18uF3s?P001DDNadVB>`Mwv:00Rf,0*42
!AIVDM,2,1,9,A,53o=>V82Duqhh@hL000A:r0I8TA@`tJ0p4q<Dp1S4pS887890@FPH0l5?gwp,0*2B
!AIVDM,2,2,9,A,?wwh0000000,2*7A
!AIVDM,1,1,,B,13aOBG@0081DMuAdVov6BrMf0<1U,0*16
!AIVDM,1,1,,A,18ue2`70001DF1idVB=RRQO008D4,0*1A
!AIVDM,1,1,,B,13cW:J3uh11C;aidd=M:I5o20d0K,0*79
!AIVDM,2,1,0,B,53o=>V82Duqhh@hL000A:r0I8TA@`tJ0p4q<Dp*/
