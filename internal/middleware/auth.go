package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/sergeii/practikum-go-url-shortener/pkg/security/sign"
	"log"
	"net/http"
	"strings"
	"time"
)

type contextKey int

const AuthContextKey contextKey = 0

const AuthUserCookieName = "auth"
const AuthUserCookieExpiration = time.Hour * 24 * 365
const UserIDLength = 8 // in bytes

var ErrInvalidCookieValue = errors.New("authentication cookie is corrupt")
var ErrInvalidUserID = errors.New("authentication cookie has invalid user id")
var ErrIncorrectCookieSig = errors.New("authentication cookie signature is not correct")

type AuthUser struct {
	ID string
}

// authenticateUser пытается аутентифицировать пользователя
// по имеющейся у него подписанной нашим секретным ключом куке
// и получить из значения куки идентификатор пользователя,
// предварительно провалидировав подпись
func authenticateUser(r *http.Request, secretKey []byte) (*AuthUser, error) {
	cookie, err := r.Cookie(AuthUserCookieName)
	if err != nil {
		return nil, err
	}

	cookieParts := strings.Split(cookie.Value, ":")
	if len(cookieParts) != 2 {
		return nil, ErrInvalidCookieValue
	}
	// Авторизационная кука состоит из собственно текстового идентификатора пользователя
	// и подписи в формате base64, разделенных двоеточием
	userID := cookieParts[0]
	// user id не может быть пустым
	if userID == "" {
		return nil, ErrInvalidUserID
	}
	cookieSig, err := base64.StdEncoding.DecodeString(cookieParts[1])
	if err != nil {
		return nil, err
	}

	signer := sign.New(secretKey)
	if !signer.Verify([]byte(userID), cookieSig) {
		return nil, ErrIncorrectCookieSig
	}

	return &AuthUser{ID: userID}, nil
}

// createNewUser генерирует уникальный идентификатор пользователя
// и возвращает структуру AuthUser с вновь созданным идентификатором
func createNewUser() (*AuthUser, error) {
	randomID := make([]byte, UserIDLength)
	_, err := rand.Read(randomID)
	if err != nil {
		log.Printf("unable to generate user id of length %v due to %v\n", UserIDLength, err)
		return nil, err
	}
	userID := hex.EncodeToString(randomID)
	return &AuthUser{ID: userID}, nil
}

// setAuthenticationCookie подписывает идентификатор пользователя секретным ключом
// и на его основе устанавливает авторизационную куку
func setAuthenticationCookie(w http.ResponseWriter, secretKey []byte, user *AuthUser) {
	signer := sign.New(secretKey)
	signature64 := signer.Sign64([]byte(user.ID))
	cookieValue := user.ID + ":" + signature64
	cookieExpiresAt := time.Now().Add(AuthUserCookieExpiration)
	cookie := http.Cookie{Name: AuthUserCookieName, Value: cookieValue, Expires: cookieExpiresAt}
	http.SetCookie(w, &cookie)
}

// WithAuthentication возвращает функцию-мидлварь для осуществления аутентификации пользователей,
// используя механизм подписанных секретным ключом кук.
// Принимает на вход секретный ключ для создания/проверки подписи по алгоритму HMAC
// Устанавливает в контекст запроса ключ со структурой AuthUser
func WithAuthentication(secretKey []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var user *AuthUser

			// Пробуем прочитать авторизационную куку, провалидировать ее подлинность
			// и в итоге получить идентификатор пользователя
			user, err := authenticateUser(r, secretKey)
			// в случае ошибки чтения пользователя из куки, логируем ее, но никак на это не реагируем,
			// позволяя коду ниже установить новую куку
			if err != nil {
				log.Printf("unable to authenticate user due to %v\n", err)
			}

			// Для анонима генерируем новый идентификатор
			if user == nil {
				user, err = createNewUser()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				log.Printf("created new user with id %s\n", user.ID)
				setAuthenticationCookie(w, secretKey, user)
			} else {
				log.Printf("autheticated existing user with id %s\n", user.ID)
			}

			// К этому моменту мы должны иметь инициализированную структуру пользователя
			// Если это не так, то в коде выше что-то пошло не так
			if user == nil {
				log.Println("user is neither authenticated nor generated")
				http.Error(w, "authentication error", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), AuthContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
