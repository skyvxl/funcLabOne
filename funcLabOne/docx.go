package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type LetterData struct {
	SenderCompanyFull     string
	SenderCompanyShort    string
	SenderContacts        string
	SenderEmail           string
	SenderOKPO            string
	SenderOGRN            string
	SenderINN             string
	SenderKPP             string
	OutDate               string
	OutNumber             string
	InNumber              string
	InDate                string
	RecipientPost         string
	RecipientOrganization string
	RecipientName         string
	GreetingName          string
	LetterSubject         string
	LetterBody            string
	SenderPost            string
	SenderName            string
}

type textNode struct {
	contentStart int
	contentEnd   int
	text         string
}

func generateLetter(templatePath, outputPath string, data LetterData) error {
	reader, err := zip.OpenReader(templatePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer output.Close()

	writer := zip.NewWriter(output)
	for _, file := range reader.File {
		header := file.FileHeader
		header.Name = file.Name
		header.Method = file.Method

		if file.FileInfo().IsDir() {
			if _, err := writer.CreateHeader(&header); err != nil {
				_ = writer.Close()
				return err
			}
			continue
		}

		bytes, err := readZipFile(file)
		if err != nil {
			_ = writer.Close()
			return err
		}
		if isWordXMLPart(file.Name) && utf8.Valid(bytes) {
			bytes = []byte(applyReplacements(string(bytes), data))
		}

		part, err := writer.CreateHeader(&header)
		if err != nil {
			_ = writer.Close()
			return err
		}
		if _, err := part.Write(bytes); err != nil {
			_ = writer.Close()
			return err
		}
	}

	return writer.Close()
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func isWordXMLPart(name string) bool {
	return strings.HasPrefix(name, "word/") &&
		strings.HasSuffix(name, ".xml") &&
		(name == "word/document.xml" ||
			strings.HasPrefix(name, "word/header") ||
			strings.HasPrefix(name, "word/footer") ||
			strings.HasPrefix(name, "word/footnotes") ||
			strings.HasPrefix(name, "word/endnotes"))
}

func applyReplacements(xmlText string, data LetterData) string {
	updated := xmlText
	for _, pair := range replacementPairs(data) {
		updated = replaceVisibleTextAll(updated, pair.target, pair.value)
	}
	return updated
}

type replacementPair struct {
	target string
	value  string
}

func replacementPairs(data LetterData) []replacementPair {
	senderRegistrationLine := fmt.Sprintf("ОКПО %s, ОГРН %s", data.SenderOKPO, data.SenderOGRN)
	senderTaxLine := fmt.Sprintf("ИНН %s, КПП %s", data.SenderINN, data.SenderKPP)

	return []replacementPair{
		{"{SENDER_COMPANY_FULL}", data.SenderCompanyFull},
		{"{SENDER_COMPANY_SHORT}", data.SenderCompanyShort},
		{"{SENDER_CONTACTS}", data.SenderContacts},
		{"{SENDER_EMAIL}", data.SenderEmail},
		{"{SENDER_OKPO}", data.SenderOKPO},
		{"{SENDER_OGRN}", data.SenderOGRN},
		{"{SENDER_INN}", data.SenderINN},
		{"{SENDER_KPP}", data.SenderKPP},
		{"{SENDER_REGISTRATION_LINE}", senderRegistrationLine},
		{"{SENDER_TAX_LINE}", senderTaxLine},
		{"{OUT_DATE}", data.OutDate},
		{"{OUT_NUMBER}", data.OutNumber},
		{"{IN_NUMBER}", data.InNumber},
		{"{IN_DATE}", data.InDate},
		{"{RECIPIENT_POST}", data.RecipientPost},
		{"{RECIPIENT_ORGANIZATION}", data.RecipientOrganization},
		{"{RECIPIENT_NAME}", data.RecipientName},
		{"{GREETING_NAME}", data.GreetingName},
		{"{LETTER_SUBJECT}", data.LetterSubject},
		{"{LETTER_BODY}", data.LetterBody},
		{"{SENDER_POST}", data.SenderPost},
		{"{SENDER_NAME}", data.SenderName},
	}
}

func replaceVisibleText(xmlText, target, replacement string) string {
	if target == "" {
		return xmlText
	}
	start, end, ok := firstMatchRange(xmlText, target)
	if !ok {
		return xmlText
	}
	return replaceVisibleTextRange(xmlText, start, end, replacement)
}

func replaceVisibleTextAll(xmlText, target, replacement string) string {
	if target == "" {
		return xmlText
	}

	nodes, ok := textNodes(xmlText)
	if !ok {
		return xmlText
	}

	var visible strings.Builder
	for _, node := range nodes {
		visible.WriteString(node.text)
	}

	var matches [][2]int
	visibleText := visible.String()
	targetLen := len([]rune(target))
	searchFrom := 0
	for {
		index := strings.Index(visibleText[searchFrom:], target)
		if index < 0 {
			break
		}
		byteStart := searchFrom + index
		start := len([]rune(visibleText[:byteStart]))
		matches = append(matches, [2]int{start, start + targetLen})
		searchFrom = byteStart + len(target)
	}

	updated := xmlText
	for i := len(matches) - 1; i >= 0; i-- {
		updated = replaceVisibleTextRange(updated, matches[i][0], matches[i][1], replacement)
	}
	return updated
}

func firstMatchRange(xmlText, target string) (int, int, bool) {
	nodes, ok := textNodes(xmlText)
	if !ok {
		return 0, 0, false
	}

	var visible strings.Builder
	for _, node := range nodes {
		visible.WriteString(node.text)
	}

	visibleText := visible.String()
	byteStart := strings.Index(visibleText, target)
	if byteStart < 0 {
		return 0, 0, false
	}
	start := len([]rune(visibleText[:byteStart]))
	return start, start + len([]rune(target)), true
}

func replaceVisibleTextRange(xmlText string, start, end int, replacement string) string {
	nodes, ok := textNodes(xmlText)
	if !ok {
		return xmlText
	}

	ranges := make([][2]int, 0, len(nodes))
	cursor := 0
	for _, node := range nodes {
		length := len([]rune(node.text))
		ranges = append(ranges, [2]int{cursor, cursor + length})
		cursor += length
	}

	replacement = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(replacement)
	nodeTexts := make([]string, len(nodes))
	for i, node := range nodes {
		nodeTexts[i] = node.text
	}

	touched := false
	for i, span := range ranges {
		overlapStart := max(start, span[0])
		overlapEnd := min(end, span[1])
		if overlapStart >= overlapEnd {
			continue
		}

		localStart := overlapStart - span[0]
		localEnd := overlapEnd - span[0]
		prefix := takeRunes(nodes[i].text, 0, localStart)
		suffix := takeRunes(nodes[i].text, localEnd, span[1]-span[0])

		if !touched {
			text := prefix + replacement
			if end <= span[1] {
				text += suffix
			}
			nodeTexts[i] = text
			touched = true
		} else if end <= span[1] {
			nodeTexts[i] = suffix
		} else {
			nodeTexts[i] = ""
		}
	}

	if !touched {
		return xmlText
	}

	var output strings.Builder
	last := 0
	for i, node := range nodes {
		output.WriteString(xmlText[last:node.contentStart])
		output.WriteString(escapeXMLText(nodeTexts[i]))
		last = node.contentEnd
	}
	output.WriteString(xmlText[last:])
	return output.String()
}

func textNodes(xmlText string) ([]textNode, bool) {
	nodes := []textNode{}
	offset := 0

	for {
		openStart, ok := findTextTag(xmlText, offset)
		if !ok {
			break
		}
		openRelEnd := strings.IndexByte(xmlText[openStart:], '>')
		if openRelEnd < 0 {
			return nil, false
		}
		openEnd := openStart + openRelEnd + 1
		closeRelStart := strings.Index(xmlText[openEnd:], "</w:t>")
		if closeRelStart < 0 {
			return nil, false
		}
		closeStart := openEnd + closeRelStart
		content := xmlText[openEnd:closeStart]
		nodes = append(nodes, textNode{
			contentStart: openEnd,
			contentEnd:   closeStart,
			text:         unescapeXMLText(content),
		})
		offset = closeStart + len("</w:t>")
	}

	return nodes, true
}

func findTextTag(xmlText string, offset int) (int, bool) {
	for {
		rel := strings.Index(xmlText[offset:], "<w:t")
		if rel < 0 {
			return 0, false
		}
		openStart := offset + rel
		tagEnd := openStart + len("<w:t")
		if tagEnd >= len(xmlText) {
			return openStart, true
		}
		switch xmlText[tagEnd] {
		case '>', ' ', '\t', '\r', '\n':
			return openStart, true
		default:
			offset = tagEnd
		}
	}
}

func takeRunes(text string, start, end int) string {
	runes := []rune(text)
	return string(runes[start:end])
}

func escapeXMLText(text string) string {
	var buffer bytes.Buffer
	_ = xml.EscapeText(&buffer, []byte(text))
	return buffer.String()
}

func unescapeXMLText(text string) string {
	replacer := strings.NewReplacer(
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&apos;", "'",
		"&amp;", "&",
	)
	return replacer.Replace(text)
}

func docxContainsText(path, needle string) (bool, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return false, err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if !isWordXMLPart(file.Name) {
			continue
		}
		bytes, err := readZipFile(file)
		if err != nil {
			return false, err
		}
		if utf8.Valid(bytes) && strings.Contains(collectVisibleText(string(bytes)), needle) {
			return true, nil
		}
	}
	return false, nil
}

func collectVisibleText(xmlText string) string {
	nodes, ok := textNodes(xmlText)
	if !ok {
		return ""
	}
	var visible strings.Builder
	for _, node := range nodes {
		visible.WriteString(node.text)
	}
	return visible.String()
}
