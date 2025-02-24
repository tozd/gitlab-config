package config

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// A reasonable subset of supported file extensions for avatar image.
// See: https://gitlab.com/gitlab-org/gitlab/-/blob/master/app/uploaders/avatar_uploader.rb
// See: https://gitlab.com/gitlab-org/gitlab/-/blob/master/lib/gitlab/file_type_detection.rb#L22
var avatarFileExtensions = []string{ //nolint:gochecknoglobals
	".png",
	".jpg",
	".jpeg",
	".gif",
	".ico",
}

// checkAvatarExtension returns an error if the provided file extension ext
// is not among allowed file extensions avatarFileExtensions.
func checkAvatarExtension(ext string) errors.E {
	for _, valid := range avatarFileExtensions {
		if valid == ext {
			return nil
		}
	}
	errE := errors.New("invalid avatar extension")
	errors.Details(errE)["ext"] = ext
	return errE
}

// getAvatar populates configuration struct with GitLab's project avatar available
// from GitLab projects API endpoint.
func (c *GetCommand) getAvatar(
	_ *gitlab.Client, project map[string]interface{}, configuration *Configuration,
) (bool, errors.E) { //nolint:unparam
	fmt.Fprintf(os.Stderr, "Getting avatar...\n")

	avatarURLAny, ok := project["avatar_url"]
	if ok && avatarURLAny != nil {
		avatarURL, ok := avatarURLAny.(string)
		if !ok {
			errE := errors.New(`"avatar_url" is not a string`)
			errors.Details(errE)["type"] = fmt.Sprintf("%T", avatarURLAny)
			errors.Details(errE)["value"] = avatarURLAny
			return false, errE
		}
		avatarExt := path.Ext(avatarURL)
		errE := checkAvatarExtension(avatarExt)
		if errE != nil {
			errE = errors.WithMessage(errE, `invalid "avatar_url"`)
			errors.Details(errE)["url"] = avatarURL
			return false, errE
		}
		// TODO: Make this work for private avatars, too.
		//       See: https://gitlab.com/gitlab-org/gitlab/-/issues/25498
		avatar, errE := downloadFile(avatarURL)
		if errE != nil {
			errE = errors.WithMessage(errE, "failed to get project avatar")
			errors.Details(errE)["url"] = avatarURL
			return false, errE
		}
		avatarPath := strings.TrimSuffix(c.Avatar, path.Ext(c.Avatar)) + avatarExt
		err := os.WriteFile(avatarPath, avatar, fileMode)
		if err != nil {
			errE := errors.WithMessage(err, "failed to save avatar")
			errors.Details(errE)["path"] = avatarPath
			return false, errE
		}
		configuration.Avatar = &avatarPath
	} else {
		noAvatar := ""
		configuration.Avatar = &noAvatar
	}

	return false, nil
}

// updateAvatar updates GitLab project's avatar using GitLab projects API endpoint
// based on the configuration struct.
func (c *SetCommand) updateAvatar(client *gitlab.Client, configuration *Configuration) errors.E {
	if configuration.Avatar == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating avatar...\n")

	if *configuration.Avatar == "" {
		u := "projects/" + gitlab.PathEscape(c.Project)

		// TODO: Make it really remove the avatar.
		//       See: https://gitlab.com/gitlab-org/gitlab/-/issues/348498
		req, err := client.NewRequest(http.MethodPut, u, map[string]interface{}{"avatar": nil}, nil)
		if err != nil {
			return errors.WithMessage(err, "failed to delete GitLab project avatar")
		}

		_, err = client.Do(req, nil)
		if err != nil {
			return errors.WithMessage(err, "failed to delete GitLab project avatar")
		}
	} else {
		file, err := os.Open(*configuration.Avatar)
		if err != nil {
			errE := errors.WithMessage(err, "failed to open GitLab project avatar file")
			errors.Details(errE)["path"] = *configuration.Avatar
			return errE
		}
		defer file.Close()
		_, filename := filepath.Split(*configuration.Avatar)
		_, _, err = client.Projects.UploadAvatar(c.Project, file, filename)
		if err != nil {
			return errors.WithMessage(err, "failed to upload GitLab project avatar")
		}
	}

	return nil
}
