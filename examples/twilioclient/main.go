package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwebrtc/go-sip-ua/pkg/account"
	"github.com/cloudwebrtc/go-sip-ua/pkg/endpoint"
	"github.com/cloudwebrtc/go-sip-ua/pkg/invite"
	"github.com/cloudwebrtc/go-sip-ua/pkg/mock"
	"github.com/cloudwebrtc/go-sip-ua/pkg/ua"
	"github.com/ghettovoice/gosip/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
)

// Purpose of this client variation is to test Twilio SIP Trunking https://www.twilio.com/docs/sip-trunking
// together with Twilio Branded Calls https://www.twilio.com/docs/branded-calls/business-quickstart

func main() {
	logger := log.NewDefaultLogrusLogger().WithPrefix("Client")

	const callReasonHeader = "X-Branded-CallReason" // Header that will be passed as call reason to called party
	callingParty := ""                              // e.164 number that has been branded in the Twilio console and set up for a SIP Trunk
	calledParty := ""                               // Number that has one of Twilio's Branded Call partenr app, e.g. https://callapp.com/
	trunkDomain := "<your sip trunk termination domain>.pstn.twilio.com"
	callReasonValue := "Whatever reason"

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	endpoint := endpoint.NewEndPoint(&endpoint.EndPointConfig{Extensions: []string{"replaces", "outbound"}, Dns: "8.8.8.8"}, logger)

	listen := "0.0.0.0:5080"
	logger.Infof("Listen => %s", listen)

	if err := endpoint.Listen("udp", listen); err != nil {
		logger.Panic(err)
	}

	if err := endpoint.Listen("tcp", listen); err != nil {
		logger.Panic(err)
	}

	ua := ua.NewUserAgent(&ua.UserAgentConfig{
		UserAgent: "Go Sip Client/1.0.0",
		Endpoint:  endpoint,
	}, logger)

	ua.InviteStateHandler = func(sess *invite.Session, req *sip.Request, resp *sip.Response, state invite.Status) {
		logger.Infof("InviteStateHandler: state => %v, type => %s", state, sess.Direction())
		if state == invite.InviteReceived {
			sess.ProvideAnswer(mock.Answer)
			sess.Accept(200)
		}
	}

	profile := account.NewProfile(callingParty, callingParty,
		&account.AuthInfo{
			Realm: trunkDomain,
		},
		1800,
	)

	target, err := parser.ParseSipUri(fmt.Sprintf("sip:%s@%s:5060;transport=udp", calledParty, trunkDomain))
	if err != nil {
		logger.Error(err)
	}

	customHeaders := []sip.Header{&sip.GenericHeader{
		HeaderName: callReasonHeader,
		Contents:   callReasonValue,
	}}

	go ua.Invite(profile, sip.SipUri{
		FUser:      sip.String{Str: calledParty},
		FHost:      target.Host(),
		FPort:      target.Port(),
		FUriParams: target.UriParams(),
	}, nil, &customHeaders)

	<-stop

	ua.Shutdown()
}
