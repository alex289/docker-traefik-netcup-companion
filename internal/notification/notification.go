package notification

import (
	"fmt"
	"log"

	"github.com/nicholas-fedor/shoutrrr"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
)

type Notifier struct {
	sender  *router.ServiceRouter
	enabled bool
}

func NewNotifier(urls []string) *Notifier {
	if len(urls) == 0 {
		return &Notifier{
			enabled: false,
		}
	}

	sender, err := shoutrrr.CreateSender(urls...)
	if err != nil {
		log.Printf("Failed to create notification sender: %v", err)
		return &Notifier{
			enabled: false,
		}
	}

	return &Notifier{
		sender:  sender,
		enabled: true,
	}
}

func (n *Notifier) SendSuccess(message string) {
	if !n.enabled {
		return
	}
	n.send(fmt.Sprintf("SUCCESS: %s", message))
}

func (n *Notifier) SendError(message string) {
	if !n.enabled {
		return
	}
	n.send(fmt.Sprintf("ERROR: %s", message))
}

func (n *Notifier) SendInfo(message string) {
	if !n.enabled {
		return
	}
	n.send(fmt.Sprintf("INFO: %s", message))
}

func (n *Notifier) send(message string) {
	errs := n.sender.Send(message, nil)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Printf("Notification error: %v", err)
		}
	}
}
