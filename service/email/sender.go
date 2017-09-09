package email

import (
	"crypto/tls"
	"fmt"
	"net/smtp"

	"github.com/jrivets/log4g"
	"github.com/pixty/console/common"
)

type (
	Sender interface {
		Send(to, subj, body string) error
	}

	sender struct {
		Config *common.ConsoleConfig `inject:""`
		logger log4g.Logger
	}
)

func NewEmailSender() *sender {
	sender := new(sender)
	sender.logger = log4g.GetLogger("pixty.EmailSender")
	return sender
}

func makeSmtpBody(from, to, subj, body string) string {
	message := ""
	message += fmt.Sprintf("From: %s\r\n", from)
	if len(to) > 0 {
		message += fmt.Sprintf("To: %s\r\n", to)
	}

	message += fmt.Sprintf("Subject: %s\r\n", subj)
	message += "\r\n" + body

	return message
}

func (s *sender) Send(to, subj, body string) error {
	// Set up authentication information.
	auth := smtp.PlainAuth(
		"",
		s.Config.EmailSmtpUser,
		s.Config.EmailSmtpPasswd,
		s.Config.EmailSmtpServer,
	)

	tlsconfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         s.Config.EmailSmtpServer,
	}

	conn, err := tls.Dial("tcp", s.Config.EmailSmtpServer+":465", tlsconfig)
	if err != nil {
		s.logger.Error("Could not dial with TLS, err=", err)
		return err
	}

	client, err := smtp.NewClient(conn, s.Config.EmailSmtpServer)
	if err != nil {
		s.logger.Error("Could not create new client, err=", err)
		return err
	}

	// step 1: Use Auth
	if err = client.Auth(auth); err != nil {
		s.logger.Error("Could not authenticate, err=", err)
		return err
	}

	// step 2: add all from and to
	if err = client.Mail(s.Config.EmailSmtpUser); err != nil {
		s.logger.Error("Could not set from, err=", err)
		return err
	}
	if err = client.Rcpt(to); err != nil {
		s.logger.Error("Could not set to err=", err)
		return err
	}

	// Data
	w, err := client.Data()
	if err != nil {
		s.logger.Error("Could not obtain Data() from client, err=", err)
		return err
	}

	_, err = w.Write([]byte(makeSmtpBody(s.Config.EmailSmtpUser, to, subj, body)))
	if err != nil {
		s.logger.Error("Could not write the body, err=", err)
		return err
	}

	err = w.Close()
	if err != nil {
		s.logger.Error("Could not close writer, err=", err)
		return err
	}

	client.Quit()
	return nil

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	//	return smtp.SendMail(
	//		s.Config.EmailSmtpServer+":465",
	//		auth,
	//		s.Config.EmailSmtpUser,
	//		[]string{to},
	//		[]byte(makeSmtpBody(s.Config.EmailSmtpUser, to, subj, body)),
	//	)
}
