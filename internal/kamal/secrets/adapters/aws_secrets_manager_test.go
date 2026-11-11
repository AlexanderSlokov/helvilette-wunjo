package adapters

// Ports test/secrets/aws_secrets_manager_adapter_test.rb at Kamal 2.12.0. The Ruby
// tests run the CLI with --account default, so the commands carry --profile default.

import (
	"strings"
	"testing"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

const awsVersion = "aws --version 2> /dev/null"

func awsBatchOutput(values ...string) string {
	return `{"SecretValues": [` + strings.Join(values, ",") + `], "Errors": []}`
}

func awsValue(name, secretString string) string {
	return `{"ARN": "arn:aws:secretsmanager:us-east-1:aaaaaaaaaaaa:secret:` + name + `", "Name": "` + name +
		`", "VersionId": "vvvv", "SecretString": ` + secretString + `, "VersionStages": ["AWSCURRENT"], "CreatedDate": "2024-01-01T00:00:00.000000"}`
}

func awsFetch(t *testing.T, command, output string, secrets []string, account, from string) ([]Secret, error) {
	t.Helper()
	return fetchWith(t, "aws_secrets_manager", map[string]localshell.Result{awsVersion: ok(""), command: ok(output)}, secrets, account, from)
}

func TestAwsFailsWhenErrorsArePresent(t *testing.T) {
	output := `{"SecretValues": [], "Errors": [
		{"SecretId": "unknown1", "ErrorCode": "ResourceNotFoundException", "Message": "Secrets Manager can't find the specified secret."},
		{"SecretId": "unknown2", "ErrorCode": "ResourceNotFoundException", "Message": "Secrets Manager can't find the specified secret."}]}`
	_, err := awsFetch(t, "aws secretsmanager batch-get-secret-value --secret-id-list unknown1 unknown2 --profile default --output json", output, []string{"unknown1", "unknown2"}, "default", "")
	assertFetchError(t, "unknown1: Secrets Manager can't find the specified secret. unknown2: Secrets Manager can't find the specified secret.", err)
}

func TestAwsFetch(t *testing.T) {
	output := awsBatchOutput(awsValue("secret", `"{\"KEY1\":\"VALUE1\", \"KEY2\":\"VALUE2\"}"`), awsValue("secret2", `"{\"KEY3\":\"VALUE3\"}"`))
	got, err := awsFetch(t, "aws secretsmanager batch-get-secret-value --secret-id-list secret secret2 --profile default --output json", output, []string{"secret", "secret2"}, "default", "")
	assertSecrets(t, []Secret{{"secret/KEY1", "VALUE1"}, {"secret/KEY2", "VALUE2"}, {"secret2/KEY3", "VALUE3"}}, got, err)
}

func TestAwsFetchWithStringValue(t *testing.T) {
	output := awsBatchOutput(awsValue("secret", `"a-string-secret"`), awsValue("secret2", `"{\"KEY2\":\"VALUE2\"}"`))
	got, err := awsFetch(t, "aws secretsmanager batch-get-secret-value --secret-id-list secret secret2 --profile default --output json", output, []string{"secret", "secret2"}, "default", "")
	assertSecrets(t, []Secret{{"secret", "a-string-secret"}, {"secret2/KEY2", "VALUE2"}}, got, err)
}

func TestAwsFetchWithFromPrefix(t *testing.T) {
	output := awsBatchOutput(awsValue("secret/KEY1", `"VALUE1"`), awsValue("secret/KEY2", `"VALUE2"`))
	got, err := awsFetch(t, "aws secretsmanager batch-get-secret-value --secret-id-list secret/KEY1 secret/KEY2 --profile default --output json", output, []string{"KEY1", "KEY2"}, "default", "secret")
	assertSecrets(t, []Secret{{"secret/KEY1", "VALUE1"}, {"secret/KEY2", "VALUE2"}}, got, err)
}

func TestAwsFetchWithBareJSONPrimitiveSecretStringValues(t *testing.T) {
	output := awsBatchOutput(awsValue("port", `"5432"`), awsValue("flag", `"true"`), awsValue("name", `"\"quoted-string\""`), awsValue("nothing", `"null"`))
	got, err := awsFetch(t, "aws secretsmanager batch-get-secret-value --secret-id-list port flag name nothing --profile default --output json", output, []string{"port", "flag", "name", "nothing"}, "default", "")
	assertSecrets(t, []Secret{{"port", "5432"}, {"flag", "true"}, {"name", "quoted-string"}, {"nothing", "null"}}, got, err)
}

func TestAwsFetchCoercesNonStringJSONFieldValuesToStrings(t *testing.T) {
	secretString := `"{\"host\":\"db.example\",\"port\":5432,\"ssl\":true,\"weight\":1.5,\"missing\":null,\"replicas\":{\"read\":\"r.example\",\"port\":5433},\"tags\":[\"prod\",\"db\"]}"`
	output := awsBatchOutput(awsValue("myapp/RailsUser", secretString))
	got, err := awsFetch(t, "aws secretsmanager batch-get-secret-value --secret-id-list myapp/RailsUser --profile default --output json", output, []string{"myapp/RailsUser"}, "default", "")
	assertSecrets(t, []Secret{
		{"myapp/RailsUser/host", "db.example"}, {"myapp/RailsUser/port", "5432"}, {"myapp/RailsUser/ssl", "true"},
		{"myapp/RailsUser/weight", "1.5"}, {"myapp/RailsUser/missing", "null"},
		{"myapp/RailsUser/replicas", `{"read":"r.example","port":5433}`}, {"myapp/RailsUser/tags", `["prod","db"]`},
	}, got, err)
}

func TestAwsFetchWithoutCLIInstalled(t *testing.T) {
	_, err := fetchWith(t, "aws_secrets_manager", map[string]localshell.Result{awsVersion: failed}, []string{"SECRET1"}, "default", "")
	assertFetchError(t, "AWS CLI is not installed", err)
}

func TestAwsFetchWithoutAccountOptionOmitsProfile(t *testing.T) {
	output := awsBatchOutput(awsValue("secret", `"{\"KEY1\":\"VALUE1\", \"KEY2\":\"VALUE2\"}"`))
	got, err := awsFetch(t, "aws secretsmanager batch-get-secret-value --secret-id-list secret --output json", output, []string{"secret"}, "", "")
	assertSecrets(t, []Secret{{"secret/KEY1", "VALUE1"}, {"secret/KEY2", "VALUE2"}}, got, err)
}
