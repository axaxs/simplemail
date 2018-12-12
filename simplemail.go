// Package simplemail provides a simple way to send emails by handling
// headers, boundaries, etc. for the user.
package simplemail

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/smtp"
	"strings"
	"time"
)

//Mailer is an interface that can be met with the implementation of the specified functions below
//TODO - should we require a validate func? ie, if mail does not meet minimum reqs to send then we return err
type Mailer interface {
	Send() error
}

// The Email object is the primary object of this package.  Fill out the fields // as needed, then call (*Email).Send to send it.  You should generally fill
// out at least From, To, and Body.  For HTML emails, simply populate the
// HTMLBody field.
// If no Username or Password are provided, it is assumed there is no
// authentication needed.
// If no Server is specified, localhost is used.
type Email struct {
	From            string
	FromName        string
	Sender          string
	ReplyTo         []string
	To              []string
	CC              []string
	BCC             []string
	Attachments     []*Attachment
	Body            string
	HTMLBody        string
	ContentType     string
	Subject         string
	Username        string
	Password        string
	Server          string
	Port            string
	Charset         string
	HostName        string
	XPriority       string
	XMSMailPriority string
	Importance      string
	XTraceID        string
}

// An Attachment object represents all fields needed for an email attachment.
// At the very least, you should populate the FileName and Contents.
type Attachment struct {
	ContentType        string
	ContentDescription string
	ContentDisposition string
	ContentID          string
	FileName           string
	Contents           []byte
}

// String() returns a full attachment section with headers as needed.
func (a *Attachment) String() string {
	if a.ContentType == "" {
		a.ContentType = "application/octet-stream"
	}
	if a.ContentDisposition == "" {
		a.ContentDisposition = "attachment"
	}
	s := fmt.Sprintf("Content-Type: %s", a.ContentType)
	if a.FileName != "" {
		s += fmt.Sprintf(`; name="%s"`, a.FileName)
	}
	s += "\r\n"
	if a.ContentID != "" {
		s += fmt.Sprintf("Content-ID: <%s>\r\n", a.ContentID)
	}
	s += fmt.Sprintf("Content-Disposition: %s; size=%d", a.ContentDisposition, len(a.Contents))
	if a.FileName != "" {
		s += `; filename="` + a.FileName + `"`
		s += "\r\nContent-Description: " + a.FileName
	}
	s += "\r\nContent-Transfer-Encoding: base64\r\n\r\n"
	s += base64.StdEncoding.EncodeToString(a.Contents) + "\r\n\r\n"
	return s
}

func genBoundary() string {
	all := "0123456789abcdef"
	res := ""
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 35; i++ {
		res += string(all[rand.Intn(len(all))])
	}
	return res
}

// GenID returns a random email ID.
func GenID() string {
	all := "0123456789ABCDEF"
	res := ""
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 32; i++ {
		res += string(all[rand.Intn(len(all))])
	}
	return res
}

// NewEmail returns a new *Email object with default Port 25 and Charset UTF-8.
func NewEmail() *Email {
	return &Email{Port: "25", Charset: "UTF-8"}
}

// SetHighPriority adds email headers so that the message is flagged as
// important to the receiver.
func (em *Email) SetHighPriority() {
	em.XPriority = "1 (Highest)"
	em.XMSMailPriority = "High"
	em.Importance = "High"
}

func (em *Email) isMultipart() bool {
	if strings.Contains(em.ContentType, "multipart") {
		return true
	}
	return false
}

func (em *Email) generateBody(boundary string) string {
	if em.Body == "" {
		return ""
	}

	m := `Content-Type: text/plain; charset="` + em.Charset + `"` + "\r\n"
	m += "MIME-Version: 1.0\r\n"
	m += "\r\n"
	m += em.Body + "\r\n\r\n"
	if boundary != "" {
		m += "--" + boundary + "\r\n"
	}
	return m
}

func (em *Email) generateHTML(boundary string) string {
	if em.HTMLBody == "" {
		return ""
	}
	m := `Content-Type: text/html; charset="` + em.Charset + `"` + "\r\n"
	m += "MIME-Version: 1.0\r\n"
	m += "Content-Transfer-Encoding: base64\r\n"
	m += "\r\n"
	m += base64.StdEncoding.EncodeToString([]byte(em.HTMLBody)) + "\r\n\r\n"
	if boundary != "" {
		m += "--" + boundary + "\r\n"
	}

	return m
}

func (em *Email) generateAttachments(boundary string) string {
	m := ""
	for _, e := range em.Attachments {
		m += e.String()
		m += "--" + boundary + "\r\n"
	}
	return m
}

// AttachFile creates an *Attachment object filling out the Contents and
// FileName fields and adds it to the Attachments list.  It also
// returns the *Attachment so that the user may set additional fields.
func (em *Email) AttachFile(fileName string) (*Attachment, error) {
	fileContents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	att := &Attachment{}
	att.Contents = fileContents
	attln := strings.Split(fileName, "/")
	att.FileName = attln[len(attln)-1]
	em.Attachments = append(em.Attachments, att)
	return att, nil
}

// String returns the text representation of the Email, as it will be sent
// over the wire.
func (em *Email) String() string {
	var fromline string
	if em.FromName != "" {
		fromline = fmt.Sprintf(`"%s" <%s>`, em.FromName, em.From)
	} else {
		fromline = em.From
	}

	var boundary string

	if em.ContentType == "" {
		if em.Body != "" && em.HTMLBody != "" {
			em.ContentType = "multipart/alternative"
		} else if em.HTMLBody != "" {
			em.ContentType = "text/html"
		} else {
			em.ContentType = "text/plain"
		}
	}

	m := fmt.Sprintf("Content-Type: %s", em.ContentType)

	if em.isMultipart() {
		boundary = genBoundary()
		m += fmt.Sprintf("; boundary=\"%s\"", boundary)
	}
	m += "\r\n"
	m += "MIME-Version: 1.0\r\n"
	m += fmt.Sprintf("From: %s\r\n", fromline)
	if em.Sender != "" {
		m += fmt.Sprintf("Sender: %s\r\n", em.Sender)
	}
	if len(em.ReplyTo) > 0 {
		m += fmt.Sprintf("Reply-To: %s\r\n", strings.Join(em.ReplyTo, ", "))
	}
	m += fmt.Sprintf("To: %s\r\n", strings.Join(em.To, ", "))
	if len(em.CC) > 0 {
		m += fmt.Sprintf("CC: %s\r\n", strings.Join(em.CC, ", "))
	}
	m += fmt.Sprintf("Subject: %s\r\n", em.Subject)

	m += fmt.Sprintf("Date: %s\r\n", time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700"))

	// Add priority headers only if they aren't blank
	if em.XPriority != "" {
		m += fmt.Sprintf("X-Priority: %s\r\n", em.XPriority)
	}
	if em.XMSMailPriority != "" {
		m += fmt.Sprintf("X-MSMail-Priority: %s\r\n", em.XMSMailPriority)
	}
	if em.Importance != "" {
		m += fmt.Sprintf("Importance: %s\r\n", em.Importance)
	}
	// Add traceID headers only if they aren't blank
	if em.XTraceID != "" {
		m += fmt.Sprintf("X-NSTraceID: %s\r\n", em.XTraceID)
	}

	// HostName is used to autogenerate messageids
	if em.HostName == "" {
		em.HostName = "localhost"
	}
	m += fmt.Sprintf("Message-ID: <%s@%s>\r\n", GenID(), em.HostName)

	// multipart emails must set multiple boundaries
	if em.isMultipart() {
		m += "\r\n--" + boundary + "\r\n"
	}

	if em.Body != "" && em.HTMLBody != "" && em.ContentType != "multipart/alternative" {
		b2 := genBoundary()
		m += fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", b2)
		m += "--" + b2 + "\r\n"
		m += em.generateBody(b2)
		m += em.generateHTML(b2)
		m = strings.TrimRight(m, "\r\n") + "--\r\n\r\n"
		m += "--" + boundary + "\r\n"
	} else {
		m += em.generateBody(boundary)
		m += em.generateHTML(boundary)
	}
	m += em.generateAttachments(boundary)
	if em.isMultipart() {
		m = strings.TrimRight(m, "\r\n") + "--\r\n"
	}

	return m
}

// Send sends the Email.
func (em *Email) Send() error {
	toline := append(em.To, em.CC...)
	toline = append(toline, em.BCC...)
	auth := smtp.PlainAuth("", em.Username, em.Password, em.Server)
	err := SendMail(em.Server+":"+em.Port, auth, em.From, toline, []byte(em.String()))
	if err != nil {
		return err
	}
	return nil
}
