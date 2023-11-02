package provider

/*
	Usage:
	```
	provider "uptimerobot" {
	  api_key = "[YOUR MAIN API KEY]"
	}
	```
*/

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	uptimerobotapi "github.com/vexxhost/terraform-provider-uptimerobot/internal/provider/api"
)

// Provider returns a schema.Provider.
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("UPTIMEROBOT_API_KEY", nil),
			},
			"cache_ttl": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  3600,
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"uptimerobot_account":       dataSourceAccount(),
			"uptimerobot_alert_contact": dataSourceAlertContact(),
		},
		ResourcesMap: map[string]*schema.Resource{
			"uptimerobot_alert_contact": resourceAlertContact(),
			"uptimerobot_monitor":       resourceMonitor(),
			"uptimerobot_status_page":   resourceStatusPage(),
		},
		ConfigureFunc: func(r *schema.ResourceData) (interface{}, error) {
			config := uptimerobotapi.New(r.Get("api_key").(string), r.Get("cache_ttl").(int))
			return config, nil
		},
	}
}
