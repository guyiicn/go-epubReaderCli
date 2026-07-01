package server

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	UserID               string `json:"userId"`
	AccessToken          string `json:"accessToken"`
	RefreshToken         string `json:"refreshToken"`
	AccessTokenExpiresAt int64  `json:"accessTokenExpiresAt"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type DeviceRegistration struct {
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

type Device struct {
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

type APIError struct {
	Status int
	Title  string
	Detail string
	Type   string
}

func (e APIError) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	if e.Title != "" {
		return e.Title
	}
	return "server error"
}

type ListResult[T any] struct {
	Items  []T
	Cursor int64
}

type SyncBook struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Author        string  `json:"author"`
	Format        string  `json:"format"`
	ContentHash   string  `json:"contentHash"`
	TotalChapters int     `json:"totalChapters"`
	DeletedAt     *int64  `json:"deletedAt"`
	AddedAt       *int64  `json:"addedAt"`
	UpdatedAt     *int64  `json:"updatedAt"`
	CoverURL      *string `json:"coverUrl"`
}

type BookUploadMetadata struct {
	Title         string `json:"title"`
	Author        string `json:"author"`
	Format        string `json:"format"`
	TotalChapters int    `json:"totalChapters"`
	AddedAt       int64  `json:"addedAt,omitempty"`
}

type SyncProgress struct {
	BookID           string  `json:"bookId"`
	Locator          string  `json:"locator"`
	TotalProgression float64 `json:"totalProgression"`
	UpdatedAt        int64   `json:"updatedAt"`
	DeviceID         string  `json:"deviceId"`
	UpdatedBy        string  `json:"updatedBy"`
}

type ProgressUpsert struct {
	Locator          string  `json:"locator"`
	TotalProgression float64 `json:"totalProgression"`
	UpdatedAt        int64   `json:"updatedAt"`
	DeviceID         string  `json:"deviceId"`
}

type SyncBookmark struct {
	ID        string `json:"id"`
	BookID    string `json:"bookId"`
	Locator   string `json:"locator"`
	Note      string `json:"note"`
	Color     string `json:"color"`
	CreatedAt int64  `json:"createdAt"`
	CreatedBy string `json:"createdBy"`
	UpdatedAt int64  `json:"updatedAt"`
	DeletedAt *int64 `json:"deletedAt"`
	DeviceID  string `json:"deviceId"`
}

type BookmarkUpsert struct {
	ID        string `json:"id"`
	BookID    string `json:"bookId"`
	Locator   string `json:"locator"`
	Note      string `json:"note"`
	Color     string `json:"color"`
	CreatedAt int64  `json:"createdAt"`
	DeviceID  string `json:"deviceId"`
}

type SyncAnnotation struct {
	ID           string `json:"id"`
	BookID       string `json:"bookId"`
	Locator      string `json:"locator"`
	SelectedText string `json:"selectedText"`
	Note         string `json:"note"`
	Color        string `json:"color"`
	CreatedAt    int64  `json:"createdAt"`
	CreatedBy    string `json:"createdBy"`
	UpdatedAt    int64  `json:"updatedAt"`
	DeletedAt    *int64 `json:"deletedAt"`
	DeviceID     string `json:"deviceId"`
}

type AnnotationUpsert struct {
	ID           string `json:"id"`
	BookID       string `json:"bookId"`
	Locator      string `json:"locator"`
	SelectedText string `json:"selectedText"`
	Note         string `json:"note,omitempty"`
	Color        string `json:"color"`
	CreatedAt    int64  `json:"createdAt"`
	DeviceID     string `json:"deviceId"`
}

type SearchResult struct {
	Query            string       `json:"query"`
	Total            int          `json:"total"`
	Page             int          `json:"page"`
	HasNext          bool         `json:"hasNext"`
	HasPrev          bool         `json:"hasPrev"`
	Books            []SearchBook `json:"books"`
	AlreadyInLibrary []string     `json:"alreadyInLibrary"`
}

type SearchBook struct {
	Index       int     `json:"index"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Year        *string `json:"year"`
	Language    *string `json:"language"`
	Format      string  `json:"format"`
	Size        string  `json:"size"`
	BookCommand string  `json:"bookCommand"`
	CoverURL    *string `json:"coverUrl"`
}

type SearchDownloadRequest struct {
	BookCommand string `json:"bookCommand"`
	Title       string `json:"title"`
	Author      string `json:"author,omitempty"`
}
