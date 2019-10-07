package ssm

import (
	"errors"
	"fmt"
	"github.com/mumoshu/vals/pkg/values/api"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type provider struct {
	// Keeping track of SSM services since we need a SSM service per region
	ssmClient *ssm.SSM

	// Adding caching for SSM parameters since templates are rendered twice and would do 2x network calls
	paramsCache map[string]string
	mapCache    map[string]map[string]interface{}

	// AWS SSM Parameter store global configuration
	Region string

	Format string
}

func New(cfg api.StaticConfig) *provider {
	p := &provider{
		paramsCache: map[string]string{},
		mapCache:    map[string]map[string]interface{}{},
	}
	p.Region = cfg.String("region")
	return p
}

// Get gets an AWS SSM Parameter Store value
func (p *provider) GetString(key string) (string, error) {
	if cachedVal, ok := p.paramsCache[key]; ok && strings.TrimSpace(cachedVal) != "" {
		return cachedVal, nil
	}

	ssmClient := p.getSSMClient()

	in := ssm.GetParameterInput{
		Name:           aws.String(key),
		WithDecryption: aws.Bool(true),
	}
	out, err := ssmClient.GetParameter(&in)
	if err != nil {
		return "", fmt.Errorf("get parameter: %v", err)
	}

	if out.Parameter == nil {
		return "", errors.New("datasource.ssm.Get() out.Parameter is nil")
	}

	if out.Parameter.Value == nil {
		return "", errors.New("datasource.ssm.Get() out.Parameter.Value is nil")
	}

	// Cache the value
	p.paramsCache[key] = *out.Parameter.Value
	val := p.paramsCache[key]

	p.debugf("SSM: successfully retrieved key=%s", key)

	return val, nil
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	if cachedVal, ok := p.mapCache[key]; ok {
		return cachedVal, nil
	}

	ssmClient := p.getSSMClient()

	res := map[string]interface{}{}

	in := ssm.GetParametersByPathInput{
		Path:           aws.String(key),
		WithDecryption: aws.Bool(true),
	}
	out, err := ssmClient.GetParametersByPath(&in)
	if err != nil {
		return nil, fmt.Errorf("ssm: get parameters by path: %v", err)
	}

	if len(out.Parameters) == 0 {
		return nil, errors.New("ssm: out.Parameters is empty")
	}

	for _, param := range out.Parameters {
		name := *param.Name
		name = name[len(key)+1:]
		res[name] = *param.Value
	}

	// Cache the value
	p.mapCache[key] = res
	val := p.mapCache[key]

	p.debugf("SSM: successfully retrieved key=%s", key)

	return val, nil
}

func (p *provider) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func (p *provider) getSSMClient() *ssm.SSM {
	if p.ssmClient != nil {
		return p.ssmClient
	}

	var cfg *aws.Config
	if p.Region != "" {
		cfg = aws.NewConfig().WithRegion(p.Region)
	} else {
		cfg = aws.NewConfig()
	}

	sess := session.New(cfg)
	p.ssmClient = ssm.New(sess)
	return p.ssmClient
}
