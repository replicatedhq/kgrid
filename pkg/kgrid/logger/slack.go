package logger

import (
	"fmt"
	"log"
	"time"

	"github.com/fatih/color"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/slack-go/slack"
)

type SlackLogger struct {
	// slack auth
	token   string
	channel string
	client  *slack.Client

	// slack message state
	threadTS       string
	channelID      string
	initialMessage string
	threadDoneCh   chan struct{}

	// general logger stuff
	isSilent    bool
	isVerbose   bool
	printToLogs bool
}

func NewSlackLogger(loggerSpec *types.SlackLoggerSpec) Logger {
	l := &SlackLogger{}

	token, err := loggerSpec.Token.String()
	if err != nil {
		log.Println("failed to get token for slack logger", err)
		l.printToLogs = true
	} else {
		l.token = token
	}

	l.channel = loggerSpec.Channel

	if !l.isSilent && l.token != "" {
		l.client = slack.New(l.token)
	}

	return l
}

func (l *SlackLogger) Silence() {
	if l == nil {
		return
	}
	l.isSilent = true
}

func (l *SlackLogger) Verbose() {
	if l == nil {
		return
	}
	l.isVerbose = true
}

func (l *SlackLogger) StartThread(msg string, args ...interface{}) {
	if l == nil || l.isSilent {
		return
	}

	if l.printToLogs {
		log.Printf(l.initialMessage)
	}

	l.initialMessage = fmt.Sprintf(msg, args...)
	channelID, timestamp, err := l.client.PostMessage(
		l.channel,
		slack.MsgOptionText(l.initialMessage, false),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Println("failed to send slack message", err)
		l.printToLogs = true
		return
	}

	l.threadTS = timestamp
	l.channelID = channelID
	l.threadDoneCh = make(chan struct{})

	go l.monitorThread()
}

func (l *SlackLogger) FinishThread(msg string, args ...interface{}) {
	if l == nil || l.isSilent {
		return
	}

	if l.printToLogs {
		log.Printf(msg, args...)
	}

	close(l.threadDoneCh)

	_, _, _, err := l.client.UpdateMessage(
		l.channelID,
		l.threadTS,
		slack.MsgOptionText(fmt.Sprintf(msg, args...), false),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Println("failed to update main message", err)
	}
}

func (l *SlackLogger) Debug(msg string, args ...interface{}) {
	if l == nil || l.isSilent || !l.isVerbose {
		return
	}

	fmt.Printf("    ")
	fmt.Println(fmt.Sprintf(msg, args...))
	fmt.Println("")
}

func (l *SlackLogger) Info(msg string, args ...interface{}) {
	if l == nil || l.isSilent {
		return
	}

	if l.printToLogs {
		log.Printf(msg, args...)
	}

	_, _, err := l.client.PostMessage(
		l.channel,
		slack.MsgOptionText(fmt.Sprintf(msg, args...), false), // TODO: needs app slug, release sequense
		slack.MsgOptionTS(l.threadTS),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Println("failed to send slack info message", err)
	}
}

func (l *SlackLogger) ActionWithoutSpinner(msg string, args ...interface{}) {
	if l == nil || l.isSilent {
		return
	}

	if msg == "" {
		fmt.Println("")
		return
	}

	fmt.Printf("  • ")
	fmt.Println(fmt.Sprintf(msg, args...))
}

func (l *SlackLogger) ChildActionWithoutSpinner(msg string, args ...interface{}) {
	if l == nil || l.isSilent {
		return
	}

	fmt.Printf("    • ")
	fmt.Println(fmt.Sprintf(msg, args...))
}

func (l *SlackLogger) ActionWithSpinner(msg string, args ...interface{}) {
	// if l == nil || l.isSilent {
	// 	return
	// }

	// fmt.Printf("  • ")
	// fmt.Printf(msg, args...)

	// if isatty.IsTerminal(os.Stdout.Fd()) {
	// 	s := spin.New()

	// 	fmt.Printf(" %s", s.Next())

	// 	l.spinnerStopCh = make(chan bool)
	// 	l.spinnerMsg = msg
	// 	l.spinnerArgs = args

	// 	go func() {
	// 		for {
	// 			select {
	// 			case <-l.spinnerStopCh:
	// 				return
	// 			case <-time.After(time.Millisecond * 100):
	// 				fmt.Printf("\r")
	// 				fmt.Printf("  • ")
	// 				fmt.Printf(msg, args...)
	// 				fmt.Printf(" %s", s.Next())
	// 			}
	// 		}
	// 	}()
	// }
}

func (l *SlackLogger) ChildActionWithSpinner(msg string, args ...interface{}) {
	// if l == nil || l.isSilent {
	// 	return
	// }

	// fmt.Printf("    • ")
	// fmt.Printf(msg, args...)

	// if isatty.IsTerminal(os.Stdout.Fd()) {
	// 	s := spin.New()

	// 	fmt.Printf(" %s", s.Next())

	// 	l.spinnerStopCh = make(chan bool)
	// 	l.spinnerMsg = msg
	// 	l.spinnerArgs = args

	// 	go func() {
	// 		for {
	// 			select {
	// 			case <-l.spinnerStopCh:
	// 				return
	// 			case <-time.After(time.Millisecond * 100):
	// 				fmt.Printf("\r")
	// 				fmt.Printf("    • ")
	// 				fmt.Printf(msg, args...)
	// 				fmt.Printf(" %s", s.Next())
	// 			}
	// 		}
	// 	}()
	// }
}

func (l *SlackLogger) FinishChildSpinner() {
	// if l == nil || l.isSilent {
	// 	return
	// }

	// green := color.New(color.FgHiGreen)

	// fmt.Printf("\r")
	// fmt.Printf("    • ")
	// fmt.Printf(l.spinnerMsg, l.spinnerArgs...)
	// green.Printf(" ✓")
	// fmt.Printf("  \n")

	// if isatty.IsTerminal(os.Stdout.Fd()) {
	// 	l.spinnerStopCh <- true
	// 	close(l.spinnerStopCh)
	// }
}

func (l *SlackLogger) FinishSpinner() {
	// if l == nil || l.isSilent {
	// 	return
	// }

	// green := color.New(color.FgHiGreen)

	// fmt.Printf("\r")
	// fmt.Printf("  • ")
	// fmt.Printf(l.spinnerMsg, l.spinnerArgs...)
	// green.Printf(" ✓")
	// fmt.Printf("  \n")

	// if isatty.IsTerminal(os.Stdout.Fd()) {
	// 	l.spinnerStopCh <- true
	// 	close(l.spinnerStopCh)
	// }
}

func (l *SlackLogger) FinishSpinnerWithError() {
	// if l == nil || l.isSilent {
	// 	return
	// }

	// red := color.New(color.FgHiRed)

	// fmt.Printf("\r")
	// fmt.Printf("  • ")
	// fmt.Printf(l.spinnerMsg, l.spinnerArgs...)
	// red.Printf(" ✗")
	// fmt.Printf("  \n")

	// if isatty.IsTerminal(os.Stdout.Fd()) {
	// 	l.spinnerStopCh <- true
	// 	close(l.spinnerStopCh)
	// }
}

func (l *SlackLogger) Error(err error) {
	if l == nil || l.isSilent {
		return
	}

	c := color.New(color.FgHiRed)
	c.Printf("  • ")
	c.Println(fmt.Sprintf("%#v", err))
}

func (l *SlackLogger) monitorThread() {
	spinners := []string{"|", "/", "--", "\\", "|", "/", "--", "\\"}
	spinnerIdx := 0
	delayPeriod := 5 * time.Second
	for {
		select {
		case <-l.threadDoneCh:
			return
		case <-time.After(delayPeriod):
			msg := fmt.Sprintf("%s %s", l.initialMessage, spinners[spinnerIdx])
			_, _, _, err := l.client.UpdateMessage(
				l.channelID,
				l.threadTS,
				slack.MsgOptionText(msg, false),
				slack.MsgOptionAsUser(true),
			)
			spinnerIdx = (spinnerIdx + 1) % len(spinners)

			if err == nil {
				delayPeriod = 5 * time.Second
			} else {
				log.Println("failed to update spinner", err)
				delayPeriod = 15 * time.Second
			}
		}
	}
}
