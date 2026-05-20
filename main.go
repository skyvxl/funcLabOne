package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const templateFile = "Шаблон.docx"

type section int

const (
	sectionSender section = iota
	sectionRecipient
	sectionLetter
	sectionAppendices
	sectionCount
)

var sectionTitles = [sectionCount]string{
	"F1: Отправитель",
	"F2: Получатель",
	"F3: Письмо",
	"F4: Приложения",
}

var sectionBorderTitles = [sectionCount]string{
	" Информация об отправителе ",
	" Информация о получателе ",
	" Письмо (* — обязательные поля) ",
	" Приложения к письму ",
}

type appState struct {
	app        *tview.Application
	formSlot   *tview.Flex
	forms      [sectionCount]*tview.Form
	tabBar     *tview.TextView
	status     *tview.TextView
	fields     map[string]*tview.InputField
	appendices []appendixFields
	current    section
}

type appendixFields struct {
	title *tview.InputField
	body  *tview.InputField
	pages *tview.InputField
}

type fieldSpec struct {
	key      string
	label    string
	value    string
	width    int
	required bool
}

type validationError struct {
	sec     section
	message string
}

func main() {
	app := tview.NewApplication()
	state := &appState{
		app:    app,
		fields: make(map[string]*tview.InputField),
	}

	buildSenderForm(state)
	buildRecipientForm(state)
	buildLetterForm(state)
	buildAppendicesForm(state)

	tabBar := tview.NewTextView().SetDynamicColors(true)
	state.tabBar = tabBar

	formSlot := tview.NewFlex()
	formSlot.AddItem(state.forms[sectionSender], 0, 1, true)
	state.formSlot = formSlot

	statusBar := tview.NewTextView().SetDynamicColors(true)
	state.status = statusBar

	state.current = sectionSender
	updateTabBar(state)
	setHint(state)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyF1:
			switchSection(state, sectionSender)
			return nil
		case tcell.KeyF2:
			switchSection(state, sectionRecipient)
			return nil
		case tcell.KeyF3:
			switchSection(state, sectionLetter)
			return nil
		case tcell.KeyF4:
			switchSection(state, sectionAppendices)
			return nil
		case tcell.KeyUp:
			moveFormFocus(state.forms[state.current], -1)
			return nil
		case tcell.KeyDown:
			moveFormFocus(state.forms[state.current], 1)
			return nil
		}
		return event
	})

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tabBar, 1, 0, false).
		AddItem(formSlot, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	if err := app.SetRoot(root, true).EnableMouse(true).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func buildSenderForm(state *appState) {
	f := newSectionForm(state, sectionSender)
	addFields(f, state, senderFields())
	f.AddButton("Вперёд: Получатель →", func() { switchSection(state, sectionRecipient) })
	f.AddButton("Сформировать DOCX", func() { doGenerate(state) })
	f.AddButton("Выход", func() { state.app.Stop() })
	state.forms[sectionSender] = f
}

func buildRecipientForm(state *appState) {
	f := newSectionForm(state, sectionRecipient)
	addFields(f, state, recipientFields())
	f.AddButton("← Отправитель", func() { switchSection(state, sectionSender) })
	f.AddButton("Вперёд: Письмо →", func() { switchSection(state, sectionLetter) })
	f.AddButton("Сформировать DOCX", func() { doGenerate(state) })
	f.AddButton("Выход", func() { state.app.Stop() })
	state.forms[sectionRecipient] = f
}

func buildLetterForm(state *appState) {
	f := newSectionForm(state, sectionLetter)
	addFields(f, state, letterFields())
	f.AddButton("← Получатель", func() { switchSection(state, sectionRecipient) })
	f.AddButton("Вперёд: Приложения →", func() { switchSection(state, sectionAppendices) })
	f.AddButton("Сформировать DOCX", func() { doGenerate(state) })
	f.AddButton("Выход", func() { state.app.Stop() })
	state.forms[sectionLetter] = f
}

func buildAppendicesForm(state *appState) {
	f := newSectionForm(state, sectionAppendices)
	// No static fields — appendices are added dynamically.
	f.AddButton("← Письмо", func() { switchSection(state, sectionLetter) })
	f.AddButton("+ Добавить приложение", func() {
		addAppendixFields(state)
		setHint(state)
	})
	f.AddButton("- Удалить приложение", func() {
		removeAppendixFields(state)
		setHint(state)
	})
	f.AddButton("Сформировать DOCX", func() { doGenerate(state) })
	f.AddButton("Выход", func() { state.app.Stop() })
	state.forms[sectionAppendices] = f
}

func newSectionForm(_ *appState, s section) *tview.Form {
	f := tview.NewForm()
	f.SetBorder(true).SetTitle(sectionBorderTitles[s])
	f.SetButtonsAlign(tview.AlignLeft)
	return f
}

func senderFields() []fieldSpec {
	return []fieldSpec{
		{"sender_company_full", "Полное название организации", "Акционерное общество «Инексбанк»", 68, false},
		{"sender_company_short", "Краткое название", "АКБ «Инексбанк»", 68, false},
		{"sender_contacts", "Контакты", "ул. Лубянка, 10, стр. 2, Москва, тел. (095) 228 09 78, факс 228 09 79", 68, false},
		{"sender_email", "E-mail", "ineks@msk.ru", 68, false},
		{"sender_okpo", "ОКПО", "64609789", 24, false},
		{"sender_ogrn", "ОГРН", "1097654389076", 24, false},
		{"sender_inn", "ИНН", "7701234567", 24, false},
		{"sender_kpp", "КПП", "770605405", 24, false},
		{"out_date", "Дата исходящего", "01.09.2003", 24, false},
		{"out_number", "Номер исходящего", "1-5/29", 24, false},
		{"in_number", "Номер входящего", "2381529", 24, false},
		{"in_date", "Дата входящего", "01.09.2003", 24, false},
		{"sender_post", "Должность подписанта", "Конкурсный управляющий", 68, false},
		{"sender_name", "ФИО подписанта", "А.В. Ростовцев", 68, false},
	}
}

func recipientFields() []fieldSpec {
	return []fieldSpec{
		{"recipient_post", "Должность адресата", "Генеральному директору", 68, false},
		{"recipient_organization", "Организация адресата", "ООО «Художественная школа имени Репина»", 68, false},
		{"recipient_name", "ФИО адресата (дательный падеж)", "Стрешневу Игорю Корнеевичу", 68, true},
		{"greeting_name", "Обращение (имя-отчество)", "Игорь Корнеевич", 68, false},
	}
}

func letterFields() []fieldSpec {
	return []fieldSpec{
		{"letter_subject", "Тема письма", "О погашении задолженности", 68, true},
		{"letter_body", "Текст письма", "Сообщаем Вам, что по состоянию на указанную дату организация является дебитором банка. Просим Вас письменно известить нас о сроках погашения долга.", 68, true},
	}
}

func addFields(f *tview.Form, state *appState, specs []fieldSpec) {
	for _, spec := range specs {
		label := spec.label
		if spec.required {
			label += " *"
		}
		field := tview.NewInputField().
			SetLabel(label + ": ").
			SetText(spec.value).
			SetFieldWidth(spec.width)
		state.fields[spec.key] = field
		f.AddFormItem(field)
	}
}

func addAppendixFields(state *appState) {
	f := state.forms[sectionAppendices]
	n := len(state.appendices) + 1
	pfx := fmt.Sprintf("Прил. %d", n)

	title := tview.NewInputField().SetLabel(pfx + " — Заголовок: ").SetFieldWidth(60)
	body := tview.NewInputField().SetLabel(pfx + " — Текст: ").SetFieldWidth(60)
	pages := tview.NewInputField().SetLabel(pfx + " — Листов (необяз.): ").SetFieldWidth(8)

	f.AddFormItem(title)
	f.AddFormItem(body)
	f.AddFormItem(pages)
	state.appendices = append(state.appendices, appendixFields{title, body, pages})
}

func removeAppendixFields(state *appState) {
	if len(state.appendices) == 0 {
		return
	}
	f := state.forms[sectionAppendices]
	n := f.GetFormItemCount()
	f.RemoveFormItem(n - 1)
	f.RemoveFormItem(n - 2)
	f.RemoveFormItem(n - 3)
	state.appendices = state.appendices[:len(state.appendices)-1]
}

func switchSection(state *appState, s section) {
	state.current = s
	state.formSlot.Clear()
	state.formSlot.AddItem(state.forms[s], 0, 1, true)
	updateTabBar(state)
	state.app.SetFocus(state.forms[s])
}

func updateTabBar(state *appState) {
	var sb strings.Builder
	for i, title := range sectionTitles {
		if section(i) == state.current {
			fmt.Fprintf(&sb, " [black:yellow:b] %s [-:-:-] ", title)
		} else {
			fmt.Fprintf(&sb, " [gray] %s [-] ", title)
		}
	}
	state.tabBar.SetText(sb.String())
}

func setHint(state *appState) {
	n := len(state.appendices)
	var msg string
	if n > 0 {
		msg = fmt.Sprintf("Приложений: %d. Нажмите «Сформировать DOCX» или F1–F4 для смены раздела.", n)
	} else {
		msg = "Используйте F1–F4 для переключения разделов. * — обязательные поля."
	}
	state.status.SetTextColor(tcell.ColorGray).SetText(msg)
}

func validate(state *appState) []validationError {
	var errs []validationError

	check := func(key, label string, sec section) {
		if strings.TrimSpace(state.fields[key].GetText()) == "" {
			errs = append(errs, validationError{sec, "«" + label + "» не заполнено"})
		}
	}

	check("letter_subject", "Тема письма", sectionLetter)
	check("letter_body", "Текст письма", sectionLetter)
	check("recipient_name", "ФИО адресата", sectionRecipient)

	for i, af := range state.appendices {
		if strings.TrimSpace(af.title.GetText()) == "" {
			errs = append(errs, validationError{
				sectionAppendices,
				fmt.Sprintf("Приложение %d: заголовок не заполнен", i+1),
			})
		}
	}

	return errs
}

func doGenerate(state *appState) {
	if errs := validate(state); len(errs) > 0 {
		switchSection(state, errs[0].sec)
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.message
		}
		state.status.SetTextColor(tcell.ColorRed).
			SetText("[red]Ошибка валидации: " + strings.Join(msgs, "; ") + "[-]")
		return
	}

	path, err := generateFromForm(state)
	if err != nil {
		state.status.SetTextColor(tcell.ColorRed).SetText("[red]Ошибка: " + err.Error() + "[-]")
		return
	}
	state.status.SetTextColor(tcell.ColorGreen).SetText("[green]Готово: " + path + "[-]")
}

func generateFromForm(state *appState) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	templatePath := filepath.Join(cwd, templateFile)
	if _, err := os.Stat(templatePath); err != nil {
		return "", fmt.Errorf("не найден шаблон %s", templatePath)
	}

	if err := ensureTemplatePlaceholders(templatePath); err != nil {
		return "", fmt.Errorf("обновление шаблона: %w", err)
	}

	outputDir := filepath.Join(cwd, "generated")
	outputPath := uniqueOutputPath(outputDir)
	data := collectData(state)

	if err := generateLetter(templatePath, outputPath, data); err != nil {
		return "", err
	}
	ok, err := docxContainsText(outputPath, data.LetterSubject)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("итоговый DOCX создан, но контрольная проверка темы не прошла")
	}

	return outputPath, nil
}

func collectData(state *appState) LetterData {
	get := func(key string) string { return state.fields[key].GetText() }

	appendices := make([]Appendix, 0, len(state.appendices))
	for _, af := range state.appendices {
		appendices = append(appendices, Appendix{
			Title: af.title.GetText(),
			Body:  af.body.GetText(),
			Pages: af.pages.GetText(),
		})
	}

	return LetterData{
		SenderCompanyFull:     get("sender_company_full"),
		SenderCompanyShort:    get("sender_company_short"),
		SenderContacts:        get("sender_contacts"),
		SenderEmail:           get("sender_email"),
		SenderOKPO:            get("sender_okpo"),
		SenderOGRN:            get("sender_ogrn"),
		SenderINN:             get("sender_inn"),
		SenderKPP:             get("sender_kpp"),
		OutDate:               get("out_date"),
		OutNumber:             get("out_number"),
		InNumber:              get("in_number"),
		InDate:                get("in_date"),
		RecipientPost:         get("recipient_post"),
		RecipientOrganization: get("recipient_organization"),
		RecipientName:         get("recipient_name"),
		GreetingName:          get("greeting_name"),
		LetterSubject:         get("letter_subject"),
		LetterBody:            get("letter_body"),
		SenderPost:            get("sender_post"),
		SenderName:            get("sender_name"),
		Appendices:            appendices,
	}
}

func uniqueOutputPath(outputDir string) string {
	base := filepath.Join(outputDir, "letter.docx")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base
	}
	for i := 1; i < 1000; i++ {
		path := filepath.Join(outputDir, fmt.Sprintf("letter_%d.docx", i))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
	}
	return filepath.Join(outputDir, "letter_latest.docx")
}

func moveFormFocus(form *tview.Form, delta int) {
	itemIndex, buttonIndex := form.GetFocusedItemIndex()
	itemCount := form.GetFormItemCount()
	total := itemCount + form.GetButtonCount()

	current := 0
	if itemIndex >= 0 {
		current = itemIndex
	} else if buttonIndex >= 0 {
		current = itemCount + buttonIndex
	}

	form.SetFocus(nextControlIndex(current, total, delta))
}

func nextControlIndex(current, total, delta int) int {
	if total <= 0 {
		return 0
	}
	next := (current + delta) % total
	if next < 0 {
		next += total
	}
	return next
}
