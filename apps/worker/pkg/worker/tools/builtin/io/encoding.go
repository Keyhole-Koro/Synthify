package io

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

type RepairArgs struct {
	Text string `json:"text"`
}

type RepairResult struct {
	RepairedText string `json:"repaired_text"`
}

var candidateEncodings = []encoding.Encoding{
	japanese.ShiftJIS,
	japanese.EUCJP,
	japanese.ISO2022JP,
	simplifiedchinese.GBK,
	simplifiedchinese.GB18030,
	traditionalchinese.Big5,
	korean.EUCKR,
	charmap.Windows1252,
	charmap.ISO8859_1,
}

func repairEncoding(text string) string {
	if utf8.ValidString(text) && !looksLikeMojibake(text) {
		return text
	}

	raw := []byte(text)
	for _, enc := range candidateEncodings {
		decoded, _, err := transform.Bytes(enc.NewDecoder(), raw)
		if err != nil {
			continue
		}
		if utf8.Valid(decoded) && !looksLikeMojibake(string(decoded)) {
			return string(decoded)
		}
	}

	var buf bytes.Buffer
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRune(raw[i:])
		if r != utf8.RuneError {
			buf.WriteRune(r)
		}
		i += size
	}
	return buf.String()
}

func looksLikeMojibake(s string) bool {
	if len(s) == 0 {
		return false
	}
	replacements := strings.Count(s, "")
	return float64(replacements)/float64(utf8.RuneCountInString(s)) > 0.1
}

func NewRepairTool() (core.Tool, error) {
	schema, err := core.IOSchemaFor[RepairArgs, RepairResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(_ context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args RepairArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		out, err := json.Marshal(RepairResult{RepairedText: repairEncoding(args.Text)})
		return out, core.Usage{}, err
	}
	return core.Tool{
		Name:        "repair_encoding",
		Description: "Fixes character encoding issues and garbled text (mojibake). Detects Shift-JIS, EUC-JP, GBK, Big5, EUC-KR, Windows-1252 and other common encodings.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}
