// Copyright 2020 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/nats-io/nats.go"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/alecthomas/kingpin.v2"
)

type pubCmd struct {
	subject string
	body    string
	req     bool
	replyTo string
	raw     bool
	hdrs    []string
	cnt     int
}

func configurePubCommand(app *kingpin.Application) {
	c := &pubCmd{}
	help := `Generic data publishing utility

When publishing multiple messages using the "count" flag
the body of the messages may use Go templates to create
multiple unique messages.

   nats pub test --count 10 "Message {{.Cnt}} @ {{.Time}}"

Available template variables are:

   .Cnt       the message number
   .TimeStamp RFC3339 format current time
   .Unix      seconds since 1970 in UTC
   .UnixNano  nano seconds since 1970 in UTC
   .Time      the current time

`
	pub := app.Command("pub", help).Action(c.publish)
	pub.Arg("subject", "Subject to subscribe to").Required().StringVar(&c.subject)
	pub.Arg("body", "Message body").Default("!nil!").StringVar(&c.body)
	pub.Flag("wait", "Wait for a reply from a service").Short('w').BoolVar(&c.req)
	pub.Flag("reply", "Sets a custom reply to subject").StringVar(&c.replyTo)
	pub.Flag("header", "Adds headers to the message").Short('H').StringsVar(&c.hdrs)
	pub.Flag("count", "Publish multiple messages").Default("1").IntVar(&c.cnt)

	req := app.Command("request", "Generic data request utility").Alias("req").Action(c.publish)
	req.Arg("subject", "Subject to subscribe to").Required().StringVar(&c.subject)
	req.Arg("body", "Message body").Default("!nil!").StringVar(&c.body)
	req.Flag("wait", "Wait for a reply from a service").Short('w').Default("true").Hidden().BoolVar(&c.req)
	req.Flag("raw", "Show just the output received").Short('r').Default("false").BoolVar(&c.raw)
	req.Flag("header", "Adds headers to the message").Short('H').StringsVar(&c.hdrs)
}

func (c *pubCmd) prepareMsg(body []byte) (*nats.Msg, error) {
	msg := nats.NewMsg(c.subject)
	msg.Reply = c.replyTo
	msg.Data = body

	return msg, parseStringsToHeader(c.hdrs, msg)
}

func (c *pubCmd) doReq(nc *nats.Conn) error {
	start := time.Now()
	if !c.raw {
		log.Printf("Sending request on %q\n", c.subject)
	}

	msg, err := c.prepareMsg([]byte(c.body))
	if err != nil {
		return err
	}

	m, err := nc.RequestMsg(msg, timeout)
	if err != nil {
		return err
	}

	if c.raw {
		fmt.Println(string(m.Data))

		return nil
	}

	log.Printf("Received on %q rtt %v", m.Subject, time.Since(start))
	if len(m.Header) > 0 {
		for h, vals := range m.Header {
			for _, val := range vals {
				log.Printf("%s: %s", h, val)
			}
		}

		fmt.Println()
	}

	fmt.Println(string(m.Data))
	if !strings.HasSuffix(string(m.Data), "\n") {
		fmt.Println()
	}

	return nil
}

func (c *pubCmd) publish(_ *kingpin.ParseContext) error {
	nc, err := newNatsConn("", natsOpts()...)
	if err != nil {
		return err
	}
	defer nc.Close()

	if c.body == "!nil!" && terminal.IsTerminal(int(os.Stdout.Fd())) {
		log.Println("Reading payload from STDIN")
		body, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		c.body = string(body)
	}

	if c.req {
		return c.doReq(nc)
	}

	type pubData struct {
		Cnt       int
		Unix      int64
		UnixNano  int64
		TimeStamp string
		Time      string
	}

	t, err := template.New("body").Parse(c.body)
	if err != nil {
		return err
	}

	if c.cnt < 1 {
		c.cnt = 1
	}

	for i := 1; i <= c.cnt; i++ {
		var body bytes.Buffer
		now := time.Now()
		err = t.Execute(&body, &pubData{
			Cnt:       i,
			Unix:      now.Unix(),
			UnixNano:  now.UnixNano(),
			TimeStamp: now.Format(time.RFC3339),
			Time:      now.Format(time.Kitchen),
		})
		if err != nil {
			return err
		}

		msg, err := c.prepareMsg(body.Bytes())
		if err != nil {
			return err
		}

		err = nc.PublishMsg(msg)
		if err != nil {
			return err
		}
		nc.Flush()

		err = nc.LastError()
		if err != nil {
			return err
		}

		log.Printf("Published %d bytes to %q\n", len(c.body), c.subject)
	}

	return nil

}
