package main

import (
	"bufio"
	"fmt"
	"github.com/hamburghammer/rcon"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"go.uber.org/atomic"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	l := logrus.New()
	app := &cli.App{
		Name:  "GRCON",
		Usage: "connect to RCON server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "address",
				Aliases: []string{"a", "addr"},
				Usage:   "RCON server address",
			},
			&cli.StringFlag{
				Name:    "password",
				Aliases: []string{"p", "pwd", "pass"},
				Usage:   "password that uses to connect to RCON server",
			},
		},
		Action: func(ctx *cli.Context) error {
			addr := ctx.String("address")
			pass := ctx.String("password")
			if addr == "" || pass == "" {
				return cli.Exit("invalid address and password", 1)
			}
			connectedChan := make(chan error)
			var con *rcon.RemoteConsole
			go func() {
				var err error
				if con, err = rcon.Dial(addr, pass); err != nil {
					connectedChan <- cli.Exit(err.Error(), 1)
				} else {
					connectedChan <- nil
				}
			}()
			wait(connectedChan, l)
			running := atomic.Bool{}
			running.Store(true)
			go func() {
				sc := bufio.NewScanner(os.Stdin)
				reg := regexp.MustCompile(`#\x1b\x5b([^\x1b]*\x7e|[\x40-\x50])#`)
				for running.Load() {
					sc.Scan()
					s := strings.ToValidUTF8(reg.ReplaceAllString(strings.TrimSpace(sc.Text()), ""), "")
					if len(s) != 0 {
						_, _ = con.Write(s)
					}
				}
			}()
			go func() {
				for running.Load() {
					resp, _, err := con.Read()
					if err != nil {
						if err.Error() == "EOF" {
							l.Error(fmt.Errorf("server: EOF"))
							running.Store(false)
							break
						}
						l.Error(err)
						continue
					}
					if resp != "" {
						lines := strings.Split(resp, "\r\n")
						for _, line := range lines {
							if line != "" {
								l.Info(line)
							}
						}
					}
				}
			}()
			go func() {
				sigs := make(chan os.Signal, 2)
				signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
				<-sigs
				running.Store(false)
			}()
			for running.Load() {
				time.Sleep(time.Second / 10)
			}
			_ = con.Close()
			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		l.Fatal(err)
	}
}

func wait(connectedChan chan error, l *logrus.Logger) {
	rec := false
	msgs := []string{".", "o", "0", "O"}
	cur := 0
	for !rec {
		select {
		case err := <-connectedChan:
			rec = true
			if err != nil {
				l.Fatal(err)
			} else {
				l.Print("Connected.")
			}
		default:
			print("Connecting " + msgs[cur] + "...\r")
			cur++
			if cur >= len(msgs) {
				cur = 0
			}
			time.Sleep(time.Second / 10)
		}
	}
}
