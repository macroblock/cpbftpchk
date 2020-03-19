package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"

	"github.com/macroblock/cpbftpchk/xftp"
	"github.com/macroblock/imed/pkg/zlog/zlog"
	"github.com/macroblock/imed/pkg/zlog/zlogger"
	"github.com/macroblock/rawin"
)

var (
	log = zlog.Instance("main")

	ftp             = xftp.IFtp(nil)
	remoteList      = []xftp.TEntry{}
	args            = ""
	diffColorPeriod = 12 * time.Hour
	refreshPeriod   = 5 * time.Minute
	refreshTime     = time.Now()
	cpbText         = ""
)

func printStat(opt *xftp.TConnStruct) {
	log.Warning(true, "<ctrl-q> quit | <ctrl-r> refresh | <ctrl-s> paste")
	log.Info(fmt.Sprintf("[%v] %v%v:%v", opt.Proto, opt.Host, opt.Path, opt.Port))
	// log.Info(fmt.Sprintf("usr:%v pwd:%v", opt.Username, opt.Password))
	log.Info("-------------------------------------------------")
}

func lookUpFile(name string, list []xftp.TEntry) *xftp.TEntry {
	for _, file := range list {
		if name == file.Name {
			return &file
		}
	}
	return nil
}

func extractFileName(s string) string {
	idx := strings.IndexFunc(s, func(r rune) bool {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.'
	})
	if idx < 0 {
		return ""
	}
	res := s[idx:]
	idx = strings.IndexFunc(res, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.')
	})
	if idx < 0 {
		idx = len(res)
	}
	return res[:idx]
}

func formatSize(size int64) string {
	if size < 0 {
		return "#err size<0"
	}
	units := " KMGTPE???"
	x := size
	for x >= 1000 {
		x /= 1000
		units = units[1:]
	}
	rest := size % 1000
	return fmt.Sprintf("%3v%v %03v", x, units[0:1], rest)
}

func formatEntry(entry *xftp.TEntry) string {
	return fmt.Sprintf("%v|%v|%v", entry.Time.Format("2006-01-02 15:04"), formatSize(entry.Size), entry.Name)
}

func reloadList(opt *xftp.TConnStruct) {
	// fmt.Printf("#### path: %q\n", opt.Path)
	log.Info("reading remote directory...")
	list, err := ftp.List(opt.Path)
	// fmt.Println("list: ", list)
	if err != nil {
		log.Warning(err, "ftp.List()")
		log.Info("reconnecting...")
		log.Warning(ftp.Quit(), "ftp.Quit()")
		ftp, err = xftp.New(*opt)
		if err != nil {
			log.Error(err, "xftp.New()")
			return
		}
		log.Info("reading remote directory...")
		list, err = ftp.List(opt.Path)
		if err != nil {
			log.Error(err, "ftp.List()")
			return
		}
	}
	remoteList = list
}

func process(ftp xftp.IFtp, opt *xftp.TConnStruct) {
	if ftp == nil {
		return
	}
	text := cpbText
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = extractFileName(line)
		if line == "" {
			continue
		}
		entry := lookUpFile(line, remoteList)
		if entry != nil {
			pre := ""
			post := ""
			if time.Since(entry.Time) > diffColorPeriod {
				pre = "\x1b[36m"
				post = "\x1b[0m"
			}
			log.Notice(pre, formatEntry(entry), post)
			continue
		}
		entry = lookUpFile(line+".part", remoteList)
		if entry != nil {
			log.Warning(true, formatEntry(entry))
			continue
		}
		log.Error(true, "                |        |", line)
	}
}

func main() {
	quit := false
	busy := true

	log.Add(zlogger.Build().Format("~x~e\n\r").Styler(zlogger.AnsiStyler).Done())

	if len(os.Args) > 1 {
		args = os.Args[1]
	}
	// opt, err := xftp.ParseConnString(args)

	log.Info("initializing...")
	opt, err := xftp.ParseConnString(args)
	if err != nil {
		log.Error(err, "xftp.ParseConnString()")
		log.Warning(true, "format:")
		log.Warning(true, "    [proto://][username[:password]@]host[/path][:port]")
		return
	}

	err = rawin.Start()
	defer rawin.Stop()
	if err != nil {
		log.Error(err, "start console raw mode")
		return
	}

	rawin.SetAction(rawin.PreFilter, func(r rune) bool { fmt.Printf("%q %U\n", r, r); return false })
	// ctrl-q
	rawin.SetAction('\x11', func(r rune) bool {
		quit = true
		return true
	})
	// ctrl-r
	rawin.SetAction('\x12', func(r rune) bool {
		if !busy {
			busy = true
			refreshTime = time.Now()
			log.Info("-------------------------------------------------")
			reloadList(opt)
			printStat(opt)
			busy = false
		}
		return true
	})
	// ctrl-s
	rawin.SetAction('\x13', func(r rune) bool {
		if !busy {
			busy = true
			err = error(nil)
			cpbText, err = clipboard.ReadAll()
			if err != nil {
				log.Error(err, "clipboard.ReadAll()")
			}
			if time.Since(refreshTime) >= refreshPeriod {
				refreshTime = time.Now()
				log.Info("-------------------------------------------------")
				reloadList(opt)
				printStat(opt)
			}
			process(ftp, opt)
			log.Info("-------------------------------------------------")
			printStat(opt)
			busy = false
		}
		return true
	})

	printStat(opt)
	log.Info("connecting...")
	ftp, err = xftp.New(*opt)
	if err != nil {
		log.Error(err, "xftp.New()")
		return
	}
	defer ftp.Quit()

	reloadList(opt)
	printStat(opt)

	busy = false
	for !quit {
		time.Sleep(50 * time.Millisecond)
		// r, err := rawin.Read()
		// if err != nil {
		// }
		// log.Info(fmt.Sprintf("--- %q", r))
	}
}
