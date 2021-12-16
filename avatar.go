package config

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/xanzy/go-gitlab"
	"gitlab.com/tozd/go/errors"
)

// A reasonable subset of supported file extensions for avatar image.
// See: https://gitlab.com/gitlab-org/gitlab/-/blob/master/app/uploaders/avatar_uploader.rb
// See: https://gitlab.com/gitlab-org/gitlab/-/blob/master/lib/gitlab/file_type_detection.rb#L22
var avatarFileExtensions = []string{
	".png",
	".jpg",
	".jpeg",
	".gif",
	".ico",
}

// checkAvatarExtension returns an error if the provided file extension ext
// is not among allowed file extensions avatarFileExtensions.
func checkAvatarExtension(ext string) error {
	for _, valid := range avatarFileExtensions {
		if valid == ext {
			return nil
		}
	}
	return errors.Errorf(`invalid avatar extension "%s"`, ext)
}

// updateAvatar updates GitLab project's avatar using GitLab projects API endpoint
// based on the configuration struct.
func updateAvatar(client *gitlab.Client, projectID string, configuration *Configuration) errors.E {
	if configuration.Avatar == "" {
		u := fmt.Sprintf("projects/%s", gitlab.PathEscape(projectID))

		// TODO: Make it really remove the avatar.
		//       See: https://gitlab.com/gitlab-org/gitlab/-/issues/348498
		req, err := client.NewRequest(http.MethodPut, u, map[string]interface{}{"avatar": nil}, nil)
		if err != nil {
			return errors.Wrap(err, `failed to delete GitLab project avatar`)
		}

		_, err = client.Do(req, nil)
		if err != nil {
			return errors.Wrap(err, `failed to delete GitLab project avatar`)
		}
	} else {
		file, err := os.Open(configuration.Avatar)
		if err != nil {
			return errors.Wrapf(err, `failed to open GitLab project avatar file "%s"`, configuration.Avatar)
		}
		defer file.Close()
		_, filename := filepath.Split(configuration.Avatar)
		_, _, err = client.Projects.UploadAvatar(projectID, file, filename)
		if err != nil {
			return errors.Wrap(err, `failed to upload GitLab project avatar`)
		}
	}

	return nil
}
