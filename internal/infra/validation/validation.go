package validation

import (
	"errors"
	"net/http"
	"unicode"

	english "github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/go-playground/validator/v10/translations/en"
	"github.com/iamolegga/valmid"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/grantsy/grantsy/internal/infra/logger"
)

func init() {
	v := validator.New()

	eng := english.New()
	uni := ut.New(eng, eng)
	trans, _ := uni.GetTranslator("en")
	if err := en.RegisterDefaultTranslations(v, trans); err != nil {
		panic(err)
	}

	valmid.SetValidator(v)
	valmid.SetErrorHandler(
		func(w http.ResponseWriter, r *http.Request, err error) {
			log := logger.FromContext(r.Context())

			var validationError validator.ValidationErrors
			if ok := errors.As(err, &validationError); !ok {
				log.Error(
					"failed to validate request, error is not expected validation error",
					"error",
					err,
				)
				httptools.BadRequest(w, r, err.Error())
				return
			}

			fields := make([]httptools.FieldError, 0, len(validationError))
			for _, e := range validationError {
				log.Debug("validation error",
					"error", e,
					"errorValue", e.Value(),
					"errorTag", e.Tag(),
					"errorType", e.Type(),
					"errorParam", e.Param(),
				)
				fields = append(fields, httptools.FieldError{
					Field:   toSnakeCase(e.Field()),
					Message: e.Translate(trans),
				})
			}

			log.Debug("validation failed", "fields", fields)
			httptools.ValidationError(w, r, fields)
		},
	)
}

// toSnakeCase converts PascalCase/camelCase to snake_case.
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
