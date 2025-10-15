package config

type (

	Config struct {
		Github	 								Github   `yaml:"github"`
		Lark	 									Lark   `yaml:"lark"`
		Telemetry								Telemetry `yaml:"telemetry"`
	}

	Github struct {
		AppId 									int64 `yaml:"app_id"`
		PrivateKeyPath         	string `yaml:"private_key_path"`
		PrivateKey         			string `yaml:"private_key"`
	}

	Lark struct {
		Secret           			string `yaml:"secret"`
		WebHookUrl        		string   `yaml:"webhook_url"`
		AppId     						string `yaml:"app_id"`
		AppSecret     				string `yaml:"app_secret"`
		GithubToEmailMap 			string `yaml:"github_to_email_map"`
	}

	Telemetry struct {
		Enabled              		bool   `yaml:"enabled"`
		OTLPEndpoint         		string `yaml:"otlp_endpoint"`
		OTLPInsecure         		bool   `yaml:"otlp_insecure"`
		Environment          		string `yaml:"environment"`
		MetricExportInterval 		int    `yaml:"metric_export_interval"`
	}
)
