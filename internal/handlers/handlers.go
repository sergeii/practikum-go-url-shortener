package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sergeii/practikum-go-url-shortener/internal/app"
	"github.com/sergeii/practikum-go-url-shortener/internal/jobs"
	"github.com/sergeii/practikum-go-url-shortener/internal/middleware"
	"github.com/sergeii/practikum-go-url-shortener/pkg/http/resp"
	"github.com/sergeii/practikum-go-url-shortener/storage"
)

type Handler struct {
	App *app.App
}

func (handler Handler) constructShortURL(shortID string) *url.URL {
	// Мы возвращаем короткую ссылку используя настройки базового URL сервиса
	baseURL := handler.App.Config.BaseURL
	shortURLPath := strings.TrimRight(baseURL.Path, "/") + "/" + shortID
	return &url.URL{
		Scheme: baseURL.Scheme,
		Host:   baseURL.Host,
		Path:   shortURLPath,
	}
}

func (handler Handler) shortenAndSaveLongURL(longURL string, r *http.Request) (*url.URL, bool, error) {
	var userID string
	if user, ok := r.Context().Value(middleware.AuthContextKey).(*middleware.AuthUser); ok {
		userID = user.ID
	}
	created := true
	// Получаем короткий идентификатор для ссылки и кладем пару в хранилище
	proposedShortID := handler.App.Shortener.Shorten(longURL)
	// При попытке сохранить уже сокращенный урл можем получить конфликт
	// и актуальный для этой ссылки короткий идентификатор
	actualShortID, err := handler.App.Storage.Set(r.Context(), proposedShortID, longURL, userID)
	if err != nil {
		if errors.Is(err, storage.ErrURLAlreadyExists) {
			created = false
		} else {
			return nil, false, err
		}
	}
	shortURL := handler.constructShortURL(actualShortID)
	return shortURL, created, nil
}

// ShortenURL принимает на вход произвольный URL в теле запроса и создает для него "короткую" версию,
// при переходе по которой пользователь попадет на оригинальный "длинный" URL
// В случае успеха возвращает код 201 и готовую короткую ссылку в теле ответа
// В случае отстуствия валидного URL в теле запроса вернет ошибку 400
// В случае наличия в хранилище сокращаемой ссылки возвращает статус 409
// и ранее сокращенную ссылку в теле ответа
func (handler Handler) ShortenURL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Пытаемся получить длинный url из тела запроса
	longURL := string(body)
	if longURL == "" {
		http.Error(w, "please provide a url to shorten", http.StatusBadRequest)
		return
	}
	respStatus := http.StatusCreated
	// В случае конфликта при сохранении отдаем статус 409 и возвращаем короткий URL для ссылки, сохраненной ранее
	shortURL, created, err := handler.shortenAndSaveLongURL(longURL, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !created {
		respStatus = http.StatusConflict
	}
	w.WriteHeader(respStatus)
	w.Write([]byte(shortURL.String())) // nolint:errcheck
}

// ExpandURL перенаправляет пользователя, перешедшего по короткой ссылке, на оригинальный "длинный" URL.
// В случае успеха возвращает код 307 с редиректом на оригинальный URL
// В случае неизвестной сервису короткой ссылки возвращает ошибку 404
func (handler Handler) ExpandURL(w http.ResponseWriter, r *http.Request) {
	// Пытаемся получить id короткой ссылки из пути
	// и найти по нему длинную ссылку, которую затем возвращаем в виде 307 редиректа
	shortURLID := chi.URLParam(r, "slug")
	if shortURLID == "" {
		http.Error(w, "Invalid short url", http.StatusBadRequest)
		return
	}
	longURL, err := handler.App.Storage.Get(r.Context(), shortURLID)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrURLNotFound):
			// Короткая ссылка не найдена в хранилище - ожидаемое поведение, возвращаем 404
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, storage.ErrURLIsDeleted):
			http.Error(w, "url is deleted", http.StatusGone)
		default:
			// Другая проблема с хранилищем
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, longURL, http.StatusTemporaryRedirect)
}

// APIShortenURL по аналогии с ShortenURL принимает на вход произвольный URL и создает для него короткую ссылку.
// Эндпоинт принимает ссылку в виде json, URL в котором указывается ключем "url"
// В случае успеха возвращает код 201 и готовую короткую ссылку в теле ответа, так же в виде json.
// В случае отстуствия валидного URL в теле запроса вернет ошибку 400
// В случае наличия в хранилище сокращаемой ссылки возвращает статус 409
// и ранее сокращенную ссылку в ответе
func (handler Handler) APIShortenURL(w http.ResponseWriter, r *http.Request) {
	var shortenReq APIShortenRequest
	// Получили невалидный json
	if err := json.NewDecoder(r.Body).Decode(&shortenReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Значение параметра невалидно
	if shortenReq.URL == "" {
		http.Error(w, "please provide a url to shorten", http.StatusBadRequest)
		return
	}
	respStatus := http.StatusCreated
	// В случае конфликта при сохранении отдаем статус 409 и возвращаем короткий URL для ссылки, сохраненной ранее
	shortURL, created, err := handler.shortenAndSaveLongURL(shortenReq.URL, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !created {
		respStatus = http.StatusConflict
	}
	resultItem := APIShortenResult{Result: shortURL.String()}
	resp.JSONResponse(&resultItem, w, respStatus)
}

// GetUserURLs возвращает полный список всех ссылок, сокращенных текущим пользователем.
// Ссылки возвращаются парами Длинный URL + Короткий URL
// В случае отсутствия ссылок у пользователя, возвращается статус 204 без тела ответа
func (handler Handler) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.AuthContextKey).(*middleware.AuthUser)
	if !ok {
		http.Error(w, "not authenticated", http.StatusForbidden)
		return
	}
	items, err := handler.App.Storage.GetURLsByUserID(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Не найдено ни одной ссылки для текущего пользователя - возвращаем 204
	if len(items) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// Серилизуем полученный результат
	jsonItems := make([]APIUserURLItem, 0, len(items))
	for shortID, longURL := range items {
		item := APIUserURLItem{
			ShortURL:    handler.constructShortURL(shortID).String(),
			OriginalURL: longURL,
		}
		jsonItems = append(jsonItems, item)
	}
	resp.JSONResponse(&jsonItems, w, http.StatusOK)
}

func (handler Handler) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	var userShortIDs []string
	user, ok := r.Context().Value(middleware.AuthContextKey).(*middleware.AuthUser)
	if !ok {
		http.Error(w, "not authenticated", http.StatusForbidden)
		return
	}
	// Получили невалидный json
	if err := json.NewDecoder(r.Body).Decode(&userShortIDs); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	backgroundJob := jobs.DeleteUserURLs(handler.App.Storage, user.ID, userShortIDs...)
	if err := handler.App.Jobs.Add(r.Context(), backgroundJob); err != nil {
		// не удалось добавить задачу в очередь в разумное время. Очередь полна?
		// просим клиента попробовать еще раз, вернув ему 503
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("OK")) // nolint:errcheck
}

// Ping проверяет статус хранилища и возвращает 200 OK в случае успешной проверки
// В случае наличия проблем с подключением или ошибкой, связанной с превышением времени ожидания ответа,
// возвращает ошибку 500
func (handler Handler) Ping(w http.ResponseWriter, r *http.Request) {
	if err := handler.App.Storage.Ping(r.Context()); err != nil {
		log.Printf("failed to ping storage because of %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK")) // nolint:errcheck
}

// APIShortenBatch принимает список URL для сокращения.
// Список для сокращения представляет собой список пар URL - Correlation ID
// При успешном выполнении операции возвращает список сокращенных ссылок
// так же в виде пар Сокращенный URL - Correlation ID, при этом
// Correlation ID для каждой ссылки соответствует значению длинной ссылки,
// которое предоставил клиент в запросе
func (handler Handler) APIShortenBatch(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.AuthContextKey).(*middleware.AuthUser)
	if !ok {
		http.Error(w, "not authenticated", http.StatusForbidden)
		return
	}
	shortenBatchReq := make([]APIShortenBatchRequestItem, 0)
	if err := json.NewDecoder(r.Body).Decode(&shortenBatchReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Собираем список для массовой вставки
	batchItems := make([]storage.BatchItem, 0, len(shortenBatchReq))
	correlationIDs := make(map[string]string)
	for _, reqItem := range shortenBatchReq {
		if reqItem.OriginalURL == "" {
			continue
		}
		shortID := handler.App.Shortener.Shorten(reqItem.OriginalURL)
		batchItem := storage.BatchItem{ShortID: shortID, LongURL: reqItem.OriginalURL, UserID: user.ID}
		batchItems = append(batchItems, batchItem)
		// запоминаем соответствие correlation и сокращаемой ссылки для последующего ответа
		correlationIDs[reqItem.CorrelationID] = reqItem.OriginalURL
	}
	// Проверяем список на пустоту здесь, поскольку некоторые урлы могли быть отсеяны при валидации
	if len(batchItems) == 0 {
		http.Error(w, "please provide a list of urls to shorten", http.StatusBadRequest)
		return
	}

	resultItems, err := handler.App.Storage.SaveBatch(r.Context(), batchItems)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shortenBatchRes := make([]APIShortenBatchResultItem, 0, len(resultItems))
	// Собираем ответ для клиента, учитывая дубли
	for correlationID, originalURL := range correlationIDs {
		actualShortID := resultItems[originalURL]
		resultItem := APIShortenBatchResultItem{
			CorrelationID: correlationID,
			ShortURL:      handler.constructShortURL(actualShortID).String(),
		}
		shortenBatchRes = append(shortenBatchRes, resultItem)
	}

	resp.JSONResponse(&shortenBatchRes, w, http.StatusCreated)
}
