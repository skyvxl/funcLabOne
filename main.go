package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const templateFile = "Шаблон.docx"

type appState struct {
	form   *tview.Form
	status *tview.TextView
	fields map[string]*tview.InputField
}

type fieldSpec struct {
	key   string
	label string
	value string
	width int
}

func main() {
	app := tview.NewApplication()
	state := newAppState()

	state.form.SetBorder(true).SetTitle(" Генератор служебного письма ")
	state.form.SetButtonsAlign(tview.AlignLeft)
	installArrowNavigation(state.form)
	state.form.AddButton("Сформировать DOCX", func() {
		path, err := generateFromForm(state)
		if err != nil {
			state.status.SetTextColor(tcell.ColorRed).SetText("Ошибка: " + err.Error())
			return
		}
		state.status.SetTextColor(tcell.ColorGreen).SetText("Готово: " + path)
	})
	state.form.AddButton("Выход", func() {
		app.Stop()
	})

	state.status.SetTextColor(tcell.ColorGray).SetText("Заполните поля и нажмите «Сформировать DOCX».")

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(state.form, 0, 1, true).
		AddItem(state.status, 2, 0, false)

	if err := app.SetRoot(layout, true).EnableMouse(true).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newAppState() *appState {
	state := &appState{
		form:   tview.NewForm(),
		status: tview.NewTextView().SetDynamicColors(true),
		fields: map[string]*tview.InputField{},
	}

	for _, spec := range defaultFields() {
		field := tview.NewInputField().
			SetLabel(spec.label + ": ").
			SetText(spec.value).
			SetFieldWidth(spec.width)
		state.fields[spec.key] = field
		state.form.AddFormItem(field)
	}

	return state
}

func installArrowNavigation(form *tview.Form) {
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			moveFormFocus(form, -1)
			return nil
		case tcell.KeyDown:
			moveFormFocus(form, 1)
			return nil
		default:
			return event
		}
	})
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

func defaultFields() []fieldSpec {
	return []fieldSpec{
		{"sender_company_full", "Полное название организации", "Акционерное общество «Инексбанк»", 72},
		{"sender_company_short", "Краткое название", "АКБ «Инексбанк»", 72},
		{"sender_contacts", "Контакты", "ул. Лубянка, 10, стр. 2, Москва, тел. (095) 228 09 78, факс 228 09 79", 72},
		{"sender_email", "E-mail", "ineks@msk.ru", 72},
		{"sender_okpo", "ОКПО", "64609789", 24},
		{"sender_ogrn", "ОГРН", "1097654389076", 24},
		{"sender_inn", "ИНН", "7701234567", 24},
		{"sender_kpp", "КПП", "770605405", 24},
		{"out_date", "Дата исходящего", "01.09.2003", 24},
		{"out_number", "Номер исходящего", "1-5/29", 24},
		{"in_number", "Номер входящего", "2381529", 24},
		{"in_date", "Дата входящего", "01.09.2003", 24},
		{"recipient_post", "Должность адресата", "Генеральному директору", 72},
		{"recipient_organization", "Организация адресата", "ООО «Художественная школа имени Репина»", 72},
		{"recipient_name", "ФИО адресата в дательном падеже", "Стрешневу Игорю Корнеевичу", 72},
		{"greeting_name", "Обращение", "Игорь Корнеевич", 72},
		{"letter_subject", "Тема письма", "О погашении задолженности", 72},
		{"letter_body", "Текст письма", "Сообщаем Вам, что по состоянию на указанную дату организация является дебитором банка. Просим Вас письменно известить нас о сроках погашения долга.", 72},
		{"sender_post", "Должность подписанта", "Конкурсный управляющий", 72},
		{"sender_name", "ФИО подписанта", "А.В. Ростовцев", 72},
	}
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
	get := func(key string) string {
		return state.fields[key].GetText()
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
	}
}

func uniqueOutputPath(outputDir string) string {
	base := filepath.Join(outputDir, "letter.docx")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base
	}
	for index := 1; index < 1000; index++ {
		path := filepath.Join(outputDir, fmt.Sprintf("letter_%d.docx", index))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
	}
	return filepath.Join(outputDir, "letter_latest.docx")
}
