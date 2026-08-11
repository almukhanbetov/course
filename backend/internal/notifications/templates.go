package notifications

import (
	"fmt"
	"html"
	"time"
)

// This file is the one place "how to phrase event X" is decided — every
// trigger elsewhere in the codebase only ever passes a type + structured
// Data, never pre-rendered text (see EnqueueInput).

type renderedInApp struct {
	Title   string
	Message string
}

func str(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	v, ok := data[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// strEsc HTML-escapes a data value before interpolating it into an email
// body — course titles, plan names etc. ultimately come from admin input,
// so this is defense in depth against HTML injection in outgoing mail.
func strEsc(data map[string]any, key string) string {
	return html.EscapeString(str(data, key))
}

func formatDate(raw string) string {
	if raw == "" {
		return "—"
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Format("02.01.2006")
	}
	return raw
}

// renderInApp fills in the notifications.title/message columns. Unknown
// types fall back to a generic line rather than erroring — a forward-
// compatible worker should never crash on a type it doesn't recognize yet.
func renderInApp(notifType string, data map[string]any) renderedInApp {
	switch notifType {
	case TypeWelcome:
		return renderedInApp{"Добро пожаловать!", "Спасибо за регистрацию на платформе. Начните обучение прямо сейчас."}
	case TypeEnrolled:
		return renderedInApp{"Вы начали курс", fmt.Sprintf("Вы записались на курс «%s».", str(data, "course_title"))}
	case TypeCourseCompleted:
		return renderedInApp{"Курс завершён!", fmt.Sprintf("Поздравляем! Вы завершили курс «%s».", str(data, "course_title"))}
	case TypeCertificateIssued:
		return renderedInApp{"Сертификат получен", fmt.Sprintf("Вам выдан сертификат за курс «%s».", str(data, "course_title"))}
	case TypeAchievementEarned:
		return renderedInApp{"Новое достижение!", fmt.Sprintf("Новое достижение: «%s»", str(data, "achievement_title"))}
	case TypePaymentPaid:
		return renderedInApp{"Оплата успешно получена", "Спасибо за оплату. Платёж успешно обработан."}
	case TypeSubscriptionActivated:
		return renderedInApp{"Подписка активирована", fmt.Sprintf("Подписка «%s» активирована до %s.", str(data, "plan_name"), formatDate(str(data, "expires_at")))}
	case TypeSubscriptionExpiring:
		return renderedInApp{"Подписка скоро закончится", fmt.Sprintf("Ваша подписка «%s» истекает %s. Продлите её, чтобы сохранить доступ.", str(data, "plan_name"), formatDate(str(data, "expires_at")))}
	case TypeSubscriptionExpired:
		return renderedInApp{"Подписка завершена", "Срок действия вашей подписки истёк. Оформите новую, чтобы продолжить доступ к курсам по подписке."}
	case TypeCourseAnnouncement:
		return renderedInApp{"Новый курс", fmt.Sprintf("Опубликован новый курс: «%s».", str(data, "course_title"))}
	case TypeCourseApproved:
		return renderedInApp{"Курс опубликован", fmt.Sprintf("Ваш курс «%s» прошёл модерацию и опубликован.", str(data, "course_title"))}
	case TypeCourseRejected:
		return renderedInApp{"Курс отправлен на доработку", fmt.Sprintf("Курс «%s» отклонён модератором: %s", str(data, "course_title"), str(data, "rejection_reason"))}
	case TypeAssignmentSubmitted:
		return renderedInApp{"Новое домашнее задание", fmt.Sprintf("Студент отправил задание «%s» на проверку (курс «%s»).", str(data, "assignment_title"), str(data, "course_title"))}
	case TypeAssignmentApproved:
		return renderedInApp{"Задание принято", fmt.Sprintf("Ваше задание «%s» по курсу «%s» принято.", str(data, "assignment_title"), str(data, "course_title"))}
	case TypeAssignmentNeedsRevision:
		return renderedInApp{"Задание требует доработки", fmt.Sprintf("Задание «%s» по курсу «%s» отправлено на доработку: %s", str(data, "assignment_title"), str(data, "course_title"), str(data, "feedback"))}
	case TypeQuestionAnswered:
		return renderedInApp{"Ответ на ваш вопрос", fmt.Sprintf("На ваш вопрос по уроку «%s» (курс «%s») ответили.", str(data, "lesson_title"), str(data, "course_title"))}
	default:
		return renderedInApp{"Уведомление", "У вас новое уведомление."}
	}
}

// emailLayout is the one reusable wrapper every email template renders
// through — "reusable" per the spec means one shared shell, not a
// full templating engine (html/template) that Stage 12's five templates
// don't need.
func emailLayout(bodyHTML string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head>
<body style="font-family:-apple-system,Segoe UI,Arial,sans-serif;background:#f4f5f7;margin:0;padding:24px;">
  <div style="max-width:560px;margin:0 auto;background:#ffffff;border-radius:8px;padding:32px;">
    <h1 style="color:#1a1a1a;font-size:20px;margin:0 0 16px;">LMS Platform</h1>
    %s
    <p style="color:#8a8f98;font-size:12px;margin-top:32px;">Это автоматическое письмо, отвечать на него не нужно.</p>
  </div>
</body></html>`, bodyHTML)
}

// renderEmail returns ok=false for event types that have no email template
// (item 9's minimum list is Welcome, Course completed, Certificate issued,
// Subscription activated, Subscription expiring soon — everything else in
// this stage is in-app only by deliberate scope choice, see the final
// report). The worker treats ok=false as "nothing to send" and marks the
// job completed rather than failing it.
func renderEmail(notifType string, data map[string]any) (subject, htmlBody string, ok bool) {
	switch notifType {
	case TypeWelcome:
		name := strEsc(data, "first_name")
		body := fmt.Sprintf(`<p>Здравствуйте, %s!</p><p>Спасибо за регистрацию на LMS Platform. Загляните в каталог курсов, чтобы начать обучение.</p>`, name)
		return "Добро пожаловать в LMS Platform", emailLayout(body), true

	case TypeCourseCompleted:
		body := fmt.Sprintf(`<p>Поздравляем!</p><p>Вы завершили курс «%s». Отличная работа!</p>`, strEsc(data, "course_title"))
		return "Курс завершён", emailLayout(body), true

	case TypeCertificateIssued:
		body := fmt.Sprintf(`<p>Вам выдан сертификат за курс «%s».</p><p>Номер сертификата: %s</p>`,
			strEsc(data, "course_title"), strEsc(data, "certificate_number"))
		return "Ваш сертификат готов", emailLayout(body), true

	case TypeSubscriptionActivated:
		body := fmt.Sprintf(`<p>Подписка «%s» активирована.</p><p>Действует до: %s</p>`,
			strEsc(data, "plan_name"), html.EscapeString(formatDate(str(data, "expires_at"))))
		return "Подписка активирована", emailLayout(body), true

	case TypeSubscriptionExpiring:
		body := fmt.Sprintf(`<p>Ваша подписка «%s» истекает %s.</p><p>Продлите её, чтобы не потерять доступ к курсам по подписке.</p>`,
			strEsc(data, "plan_name"), html.EscapeString(formatDate(str(data, "expires_at"))))
		return "Подписка скоро закончится", emailLayout(body), true

	default:
		return "", "", false
	}
}
