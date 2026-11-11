package adapters

// Ports lib/kamal/secrets/adapters/aws_secrets_manager.rb at Kamal 2.12.0.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/shell"
)

// awsSecretsManager reads secrets with `aws secretsmanager batch-get-secret-value`.
// The account, when set, is the AWS CLI profile.
type awsSecretsManager struct{ shell localshell.Shell }

type awsBatch struct {
	SecretValues []awsSecretValue `json:"SecretValues"`
	Errors       []awsError       `json:"Errors"`
}

type awsSecretValue struct {
	Name         string `json:"Name"`
	SecretString string `json:"SecretString"`
}

type awsError struct {
	SecretID string `json:"SecretId"`
	Message  string `json:"Message"`
}

func (awsSecretsManager) requiresAccount() bool { return false }
func (awsSecretsManager) login(string) error    { return nil }

func (a awsSecretsManager) checkDependencies() error {
	if !succeeds(a.shell, "aws --version 2> /dev/null") {
		return errors.New("AWS CLI is not installed")
	}
	return nil
}

func (a awsSecretsManager) fetchSecrets(secrets []string, from, account string) ([]Secret, error) {
	values, err := a.batchGet(prefixedSecrets(secrets, from), account)
	if err != nil {
		return nil, err
	}
	var results []Secret
	for _, value := range values {
		results = awsSecretResults(results, value)
	}
	return results, nil
}

func (a awsSecretsManager) batchGet(secrets []string, account string) ([]awsSecretValue, error) {
	result, err := a.shell.Run(awsBatchCommand(secrets, account))
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("Could not read %s from AWS Secrets Manager", result.Stdout)
	}
	var batch awsBatch
	if err := json.Unmarshal([]byte(result.Stdout), &batch); err != nil {
		return nil, fmt.Errorf("AWS Secrets Manager output is not JSON: %w", err)
	}
	if len(batch.Errors) > 0 {
		return nil, errors.New(awsErrorMessage(batch.Errors))
	}
	return batch.SecretValues, nil
}

func awsBatchCommand(secrets []string, account string) string {
	words := []string{"aws", "secretsmanager", "batch-get-secret-value", "--secret-id-list"}
	for _, secret := range secrets {
		words = append(words, shell.Escape(secret))
	}
	if account != "" {
		words = append(words, "--profile", shell.Escape(account))
	}
	return strings.Join(append(words, "--output", "json"), " ")
}

func awsErrorMessage(errs []awsError) string {
	messages := make([]string, len(errs))
	for i, e := range errs {
		messages[i] = e.SecretID + ": " + e.Message
	}
	return strings.Join(messages, " ")
}

// awsSecretResults adds one secret: each field of a JSON object as "name/key", any
// other JSON value as "name", and text that is not JSON as it is.
func awsSecretResults(results []Secret, value awsSecretValue) []Secret {
	text := []byte(value.SecretString)
	if !json.Valid(text) {
		return putSecret(results, value.Name, value.SecretString)
	}
	if !isJSONObject(text) {
		return putSecret(results, value.Name, stringifyJSON(text))
	}
	fields, _ := objectFields(text)
	for _, field := range fields {
		results = putSecret(results, value.Name+"/"+field.Key, stringifyJSON(field.Value))
	}
	return results
}
