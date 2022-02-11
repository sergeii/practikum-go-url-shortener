package handlers

type APIShortenRequest struct {
	URL string `json:"url"` // Оригинальный длинный URL, требующий укорачивания
}

type APIShortenResult struct {
	Result string `json:"result"` // Короткий URL, превращенный из длинного
}

type APIUserURLItem struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}
