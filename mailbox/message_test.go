package mailbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageParseBody(t *testing.T) {
	body := `Return-Path: <sender@betom.de>
Authentication-Results:  kundenserver.de; dkim=none
Received: from mout.kundenserver.de ([212.227.126.135]) by mx.kundenserver.de
 (mxeue102 [217.72.192.67]) with ESMTPS (Nemesis) id 1MgQMD-1rir0j14hj-00ojoy
 for <receiver@betom.de>; Fri, 08 Dec 2023 23:01:50 +0100
Received: from smtpclient.apple ([91.97.158.187]) by mrelayeu.kundenserver.de
 (mreue011 [212.227.15.167]) with ESMTPSA (Nemesis) id
 1MRT6b-1qqbfM3xZ7-00NRQ0 for <reseiver@betom.de>; Fri, 08 Dec 2023 23:01:50
 +0100
From: Mr. Sender <sender@betom.de>
Content-Type: multipart/alternative;
	boundary="Apple-Mail=_30EF9BCC-CBCB-4D8E-923B-4D84F807ACF2"
Mime-Version: 1.0 (Mac OS X Mail 16.0 \(3731.700.6\))
Subject: Hello
Message-Id: <8FA1FA02-1C50-41EC-BBC9-760067F01735@betom.de>
Date: Fri, 8 Dec 2023 23:01:39 +0100
To: receiver@betom.de
Envelope-To: <receiver@betom.de>


--Apple-Mail=_30EF9BCC-CBCB-4D8E-923B-4D84F807ACF2
Content-Transfer-Encoding: 7bit
Content-Type: text/plain;
	charset=us-ascii

Hello world!
--Apple-Mail=_30EF9BCC-CBCB-4D8E-923B-4D84F807ACF2
Content-Transfer-Encoding: 7bit
Content-Type: text/html;
	charset=us-ascii

<html><head><meta http-equiv="content-type" content="text/html; charset=us-ascii"></head><body style="overflow-wrap: break-word; -webkit-nbsp-mode: space; line-break: after-white-space;"><b>123</b></body></html>
--Apple-Mail=_30EF9BCC-CBCB-4D8E-923B-4D84F807ACF2--
`

	message, err := NewMessage([]byte(body))
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	subject, err := message.Subject()
	if assert.NoError(t, err) {
		assert.Equal(t, "Hello", subject)
	}

	text, err := message.PlainText()
	if assert.NoError(t, err) {
		assert.Equal(t, "Hello world!", text)
	}
}
