package user

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/akifkadioglu/vocapedia/pkg/cache"
	"github.com/akifkadioglu/vocapedia/pkg/database"
	"github.com/akifkadioglu/vocapedia/pkg/entities"
	"github.com/akifkadioglu/vocapedia/pkg/i18n"
	"github.com/akifkadioglu/vocapedia/pkg/mail"
	"github.com/akifkadioglu/vocapedia/pkg/search"
	"github.com/akifkadioglu/vocapedia/pkg/token"
	"github.com/akifkadioglu/vocapedia/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/meilisearch/meilisearch-go"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func GetByUsername(w http.ResponseWriter, r *http.Request) {
	db := database.Manager()

	username := r.URL.Query().Get("username")

	if username == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "Username query parameter is required",
		})
		return
	}

	var user entities.User
	err := db.Where("LOWER(username) = ?", strings.ToLower(username)).First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]string{
				"error": "User not found",
			})
		} else {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, map[string]string{
				"error": "Database error",
			})
		}
		return
	}

	render.JSON(w, r, user)
}

func SearchUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	client := search.Meili()
	index := client.Index("users")

	searchRequest := &meilisearch.SearchRequest{
		Query: query,
		Limit: 10,
	}

	searchRes, err := index.Search(query, searchRequest)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": "Search error: " + err.Error(),
		})
		return
	}

	render.JSON(w, r, map[string]any{
		"list": searchRes.Hits,
	})
}

func EditUser(w http.ResponseWriter, r *http.Request) {
	db := database.Manager()

	userID := token.User(r).UserID
	if userID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.user_id_required"),
		})
		return
	}
	var user entities.User
	err := db.Where("id = ?", userID).First(&user).Error
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.user_not_found"),
		})
		return
	}
	var updatedUser _updateUser
	if err := render.Decode(r, &updatedUser); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.invalid_request_body"),
		})
		return
	}
	device, err := utils.StructToMap(updatedUser.Device)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}

	deviceString := ""
	for _, k := range device {
		deviceString = deviceString + " | " + fmt.Sprintf("%v", k)
	}
	if err := db.Model(&user).Updates(map[string]any{
		"name":      updatedUser.Name,
		"username":  updatedUser.Username,
		"biography": updatedUser.Biography,
	}).Where("id = ?", userID).Error; err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.failed_to_update_user"),
		})
		return
	}

	tmpl, err := template.ParseFiles("pkg/mail/templates/edit.user.html")
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}
	var body bytes.Buffer

	err = tmpl.Execute(&body, _emailData{
		Header:      i18n.Localizer(r, "mail.edit.user.header"),
		Description: i18n.Localizer(r, "mail.edit.user.description"),
		Device:      deviceString,
	})
	if err != nil {
		render.Status(r, http.StatusInternalServerError)

		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}
	subject := i18n.Localizer(r, "mail.edit.user.header")

	isSent, err := mail.Send(r, user.Email, subject, body.String())
	if err != nil && isSent {
		render.Status(r, http.StatusBadRequest)

		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}
	jwtModel := entities.JwtModel{
		UserID:   userID,
		Username: user.Username,
		Device:   device,
	}

	tokenString, err := token.GenerateToken(jwtModel)
	if err != nil {
		render.Status(r, http.StatusBadRequest)

		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}
	render.JSON(w, r, map[string]string{
		"token": tokenString,
	})
}

func Tokens(w http.ResponseWriter, r *http.Request) {
	db := database.Manager()
	var tokenEntity []entities.Token
	tokenStr := strings.Split(r.Header.Get("Authorization"), " ")[1]
	err := db.Where("token = ?", tokenStr).First(&tokenEntity).Error
	if err != nil {
		render.JSON(w, r, map[string][]entities.Token{
			"tokens": {},
		})
		return
	}
	userID := token.User(r).UserID

	err = db.Where("user_id = ?", userID).Find(&tokenEntity).Error
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.tokens_fetch_failed"),
		})
		return
	}

	render.JSON(w, r, map[string][]entities.Token{
		"tokens": tokenEntity,
	})

}

func DeleteToken(w http.ResponseWriter, r *http.Request) {
	db := database.Manager()
	tokenIDStr := chi.URLParam(r, "id")

	userID := token.User(r).UserID
	var tokenEntity entities.Token
	if err := db.Where("id = ? AND user_id = ?", tokenIDStr, userID).First(&tokenEntity).Error; err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.token_not_found"),
		})
		return
	}

	if err := db.Delete(&tokenEntity).Error; err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}

	render.JSON(w, r, map[string]string{
		"message": i18n.Localizer(r, "success.token_deleted"),
	})
}

func Check(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]string{})
}
func UpdateVocaToken(w http.ResponseWriter, r *http.Request) {
	db := database.Manager()
	userID := token.User(r).UserID
	vocatoken, err := utils.GenerateVocaToken(10)
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}
	err = db.Model(&entities.User{}).
		Where("id = ?", userID).
		Update("vocatoken", vocatoken).Error
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}
	render.JSON(w, r, map[string]string{
		"vocatoken": vocatoken,
	})
}

func GetVocaToken(w http.ResponseWriter, r *http.Request) {
	db := database.Manager()
	userID := token.User(r).UserID
	var user entities.User
	err := db.Where("id = ?", userID).First(&user).Error
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}
	render.JSON(w, r, map[string]any{
		"vocatoken": user.Vocatoken,
	})
}

func DailyStreak(w http.ResponseWriter, r *http.Request) {
	rdb := cache.Redis()
	userID := token.User(r).UserID
	db := database.Manager()

	data, err := rdb.Get(r.Context(), fmt.Sprintf("streak:%s", userID)).Result()
	var streak _streak
	if err == redis.Nil {
		streak = _streak{Count: 0, LastDate: "", Rewarded: false}
	} else if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	} else {
		json.Unmarshal([]byte(data), &streak)
	}
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	switch streak.LastDate {
	case today:
	case yesterday:
		streak.Count++
		if streak.Count > 7 {
			streak.Count = 1
			streak.Rewarded = false
		}

		streak.LastDate = today
	default:
		streak.Count = 1
		streak.LastDate = today
		streak.Rewarded = false
	}

	if streak.Count == 7 && !streak.Rewarded {
		streak.Rewarded = true
		if err := db.Model(&entities.User{}).Where("id = ?", userID).Update("vocatoken_val", gorm.Expr("vocatoken_val + ?", 20)).Error; err != nil {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, map[string]string{
				"error": i18n.Localizer(r, "error.something_went_wrong"),
			})
			return
		}
	}

	streakJSON, _ := json.Marshal(streak)
	rdb.Set(r.Context(), fmt.Sprintf("streak:%s", userID), string(streakJSON), 0)

	render.JSON(w, r, map[string]any{
		"streak": streak,
	})
}

func RequestTeacher(w http.ResponseWriter, r *http.Request) {
	userID := token.User(r).UserID

	if userID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.user_id_required"),
		})
		return
	}

	var reqBody struct {
		Description string `json:"description"`
	}

	if err := render.DecodeJSON(r.Body, &reqBody); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.invalid_request_body"),
		})
		return
	}

	if len(reqBody.Description) < 50 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.teacher_description_too_short"),
		})
		return
	}

	// Store teacher request in Redis
	redisClient := cache.Redis()
	requestKey := fmt.Sprintf("teacher_request:%s", userID)

	// Prepare request data
	requestData := map[string]interface{}{
		"user_id":      userID,
		"description":  reqBody.Description,
		"requested_at": time.Now().Unix(),
		"status":       "pending",
	}

	requestJSON, _ := json.Marshal(requestData)

	// Set teacher request with expiration (e.g., 30 days)
	err := redisClient.Set(r.Context(), requestKey, requestJSON, 30*24*time.Hour).Err()

	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}

	render.JSON(w, r, map[string]string{
		"message": i18n.Localizer(r, "success.teacher_request_submitted"),
	})
}

// UpdateLanguagePreferences handles updating user language preferences
func UpdateLanguagePreferences(w http.ResponseWriter, r *http.Request) {
	db := database.Manager()
	userID := token.User(r).UserID

	var reqBody struct {
		KnownLanguages  []string `json:"known_languages"`
		TargetLanguages []string `json:"target_languages"`
	}

	if err := render.DecodeJSON(r.Body, &reqBody); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.invalid_request_body"),
		})
		return
	}

	var user entities.User
	if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.user_not_found"),
		})
		return
	}
	kl, _ := json.Marshal(reqBody.KnownLanguages)
	tl, _ := json.Marshal(reqBody.TargetLanguages)

	// Update language preferences
	updates := map[string]any{
		"known_languages":  datatypes.JSON(kl),
		"target_languages": datatypes.JSON(tl),
	}

	if err := db.Model(&user).Updates(updates).Error; err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.something_went_wrong"),
		})
		return
	}

	render.JSON(w, r, map[string]string{
		"message": i18n.Localizer(r, "success.language_preferences_updated"),
	})
}

// GetLanguagePreferences handles getting user language preferences
func GetLanguagePreferences(w http.ResponseWriter, r *http.Request) {
	db := database.Manager()
	userID := token.User(r).UserID

	if userID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.user_id_required"),
		})
		return
	}

	var user entities.User
	if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{
			"error": i18n.Localizer(r, "error.user_not_found"),
		})
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"known_languages":  user.KnownLanguages,
		"target_languages": user.TargetLanguages,
	})
}

// GetUserTokens returns the current user's token balance
func GetUserTokens(w http.ResponseWriter, r *http.Request) {
	userID := token.User(r).UserID
	if userID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error": "User ID required",
		})
		return
	}

	db := database.Manager()
	var user entities.User
	if err := db.Select("tokens").Where("id = ?", userID).First(&user).Error; err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{
			"error": "User not found",
		})
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"tokens": user.Tokens,
	})
}
