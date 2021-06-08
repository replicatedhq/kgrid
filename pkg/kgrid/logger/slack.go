package logger

import (
	"fmt"
	"log"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/slack-go/slack"
)

type SlackLogger struct {
	// slack auth
	token   string
	channel string
	client  *slack.Client

	// slack message state
	threadTS  string
	channelID string // why?

	// general logger stuff
	isSilent  bool
	isVerbose bool
}

func NewSlackLogger(loggerSpec *types.SlackLoggerSpec) Logger {
	l := &SlackLogger{}

	token, err := loggerSpec.Token.String()
	if err != nil {
		log.Println("failed to get token for slack logger", err)
		l.isSilent = true
	} else {
		l.token = token
	}

	channel, err := loggerSpec.Channel.String()
	if err != nil {
		log.Println("failed to get channel for slack logger", err)
		l.isSilent = true
	} else {
		l.channel = channel
	}

	if !l.isSilent {
		l.client = slack.New(l.token)

		err = l.startMessageThread()
		if err != nil {
			l.isSilent = true
			log.Println("failed to create initial slack message", err)
		}
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

func (l *SlackLogger) Initialize() {
}

func (l *SlackLogger) Finish() {
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

	_, _, err := l.client.PostMessage(
		l.channel,
		slack.MsgOptionText(fmt.Sprintf(msg, args...), false), // TODO: needs app slug, release sequense
		slack.MsgOptionTS(l.threadTS),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Println("failed to send slack message", err)
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

func (l *SlackLogger) startMessageThread() error {
	channelID, timestamp, err := l.client.PostMessage(
		l.channel,
		slack.MsgOptionText("Starting grid test", false), // TODO: needs app slug, release sequense
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		return errors.Wrap(err, "failed to send slack message")
	}

	l.threadTS = timestamp
	l.channelID = channelID
	return nil
}

// func (l *SlackLogger) callSlack(url string) ([]byte, error) {
// 	resp, err := http.Get(url)
// 	defer resp.Body.Close()

// 	body, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return body, nil
// }
