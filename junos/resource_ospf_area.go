package junos

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

type ospfAreaOptions struct {
	areaID          string
	routingInstance string
	version         string
	interFace       []map[string]interface{}
}

func resourceOspfArea() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceOspfAreaCreate,
		ReadContext:   resourceOspfAreaRead,
		UpdateContext: resourceOspfAreaUpdate,
		DeleteContext: resourceOspfAreaDelete,
		Importer: &schema.ResourceImporter{
			State: resourceOspfAreaImport,
		},
		Schema: map[string]*schema.Schema{
			"area_id": {
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
			},
			"routing_instance": {
				Type:             schema.TypeString,
				Optional:         true,
				ForceNew:         true,
				Default:          defaultWord,
				ValidateDiagFunc: validateNameObjectJunos([]string{}),
			},
			"version": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "v2",
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"v2", "v3"}, false),
			},
			"interface": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"disable": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"passive": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"metric": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntBetween(1, 65535),
						},
						"hello_interval": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntBetween(1, 255),
						},
						"retransmit_interval": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntBetween(1, 65535),
						},
						"dead_interval": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntBetween(1, 65535),
						},
					},
				},
			},
		},
	}
}

func resourceOspfAreaCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	sess.configLock(jnprSess)
	ospfAreaExists, err := checkOspfAreaExists(d.Get("area_id").(string), d.Get("version").(string),
		d.Get("routing_instance").(string), m, jnprSess)
	if err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if ospfAreaExists {
		sess.configClear(jnprSess)

		return diag.FromErr(fmt.Errorf("ospf %v area %v already exists in routing instance %v",
			d.Get("version").(string), d.Get("area_id").(string), d.Get("routing_instance").(string)))
	}
	if err := setOspfArea(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("create resource junos_ospf_area", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	ospfAreaExists, err = checkOspfAreaExists(d.Get("area_id").(string), d.Get("version").(string),
		d.Get("routing_instance").(string), m, jnprSess)
	if err != nil {
		return diag.FromErr(err)
	}
	if ospfAreaExists {
		d.SetId(d.Get("area_id").(string) + idSeparator + d.Get("version").(string) +
			idSeparator + d.Get("routing_instance").(string))
	} else {
		return diag.FromErr(fmt.Errorf("ospf %v area %v in routing instance %v not exists after commit => check your config",
			d.Get("version").(string), d.Get("area_id").(string), d.Get("routing_instance").(string)))
	}

	return resourceOspfAreaRead(ctx, d, m)
}
func resourceOspfAreaRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	mutex.Lock()
	jnprSess, err := sess.startNewSession()
	if err != nil {
		mutex.Unlock()

		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	ospfAreaOptions, err := readOspfArea(d.Get("area_id").(string), d.Get("version").(string),
		d.Get("routing_instance").(string), m, jnprSess)
	mutex.Unlock()
	if err != nil {
		return diag.FromErr(err)
	}
	if ospfAreaOptions.areaID == "" {
		d.SetId("")
	} else {
		fillOspfAreaData(d, ospfAreaOptions)
	}

	return nil
}
func resourceOspfAreaUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	d.Partial(true)
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	sess.configLock(jnprSess)
	if err := delOspfArea(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := setOspfArea(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("update resource junos_ospf_area", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	d.Partial(false)

	return resourceOspfAreaRead(ctx, d, m)
}
func resourceOspfAreaDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	sess.configLock(jnprSess)
	if err := delOspfArea(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("delete resource junos_ospf_area", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}

	return nil
}
func resourceOspfAreaImport(d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return nil, err
	}
	defer sess.closeSession(jnprSess)
	result := make([]*schema.ResourceData, 1)
	idSplit := strings.Split(d.Id(), idSeparator)
	if len(idSplit) < 3 {
		return nil, fmt.Errorf("missing element(s) in id with separator %v", idSeparator)
	}
	ospfAreaExists, err := checkOspfAreaExists(idSplit[0], idSplit[1], idSplit[2], m, jnprSess)
	if err != nil {
		return nil, err
	}
	if !ospfAreaExists {
		return nil, fmt.Errorf("don't find ospf area with id '%v' (id must be "+
			"<aread_id>"+idSeparator+"<version>"+idSeparator+"<routing_instance>)", d.Id())
	}
	ospfAreaOptions, err := readOspfArea(idSplit[0], idSplit[1], idSplit[2], m, jnprSess)
	if err != nil {
		return nil, err
	}
	fillOspfAreaData(d, ospfAreaOptions)
	result[0] = d

	return result, nil
}

func checkOspfAreaExists(idArea, version, routingInstance string,
	m interface{}, jnprSess *NetconfObject) (bool, error) {
	sess := m.(*Session)
	var ospfAreaConfig string
	var err error
	ospfVersion := opsfV2
	if version == "v3" {
		ospfVersion = ospfV3
	}
	if routingInstance == defaultWord {
		ospfAreaConfig, err = sess.command("show configuration protocols "+
			ospfVersion+" area "+idArea+" | display set", jnprSess)
		if err != nil {
			return false, err
		}
	} else {
		ospfAreaConfig, err = sess.command("show configuration routing-instances "+
			routingInstance+" protocols "+ospfVersion+" area "+idArea+" | display set", jnprSess)
		if err != nil {
			return false, err
		}
	}
	if ospfAreaConfig == emptyWord {
		return false, nil
	}

	return true, nil
}
func setOspfArea(d *schema.ResourceData, m interface{}, jnprSess *NetconfObject) error {
	sess := m.(*Session)
	configSet := make([]string, 0)
	setPrefix := setLineStart
	ospfVersion := opsfV2
	if d.Get("version").(string) == "v3" {
		ospfVersion = ospfV3
	}
	if d.Get("routing_instance").(string) == defaultWord {
		setPrefix += "protocols " + ospfVersion + " area " + d.Get("area_id").(string) + " "
	} else {
		setPrefix += "routing-instances " + d.Get("routing_instance").(string) +
			" protocols " + ospfVersion + " area " + d.Get("area_id").(string) + " "
	}
	for _, v := range d.Get("interface").([]interface{}) {
		ospfInterface := v.(map[string]interface{})
		setPrefixInterface := setPrefix + "interface " + ospfInterface["name"].(string) + " "
		if ospfInterface["disable"].(bool) {
			configSet = append(configSet, setPrefixInterface+"disable")
		}
		if ospfInterface["passive"].(bool) {
			configSet = append(configSet, setPrefixInterface+"passive")
		}
		if ospfInterface["metric"].(int) != 0 {
			configSet = append(configSet, setPrefixInterface+"metric "+
				strconv.Itoa(ospfInterface["metric"].(int)))
		}
		if ospfInterface["hello_interval"].(int) != 0 {
			configSet = append(configSet, setPrefixInterface+"hello-interval "+
				strconv.Itoa(ospfInterface["hello_interval"].(int)))
		}
		if ospfInterface["retransmit_interval"].(int) != 0 {
			configSet = append(configSet, setPrefixInterface+"retransmit-interval "+
				strconv.Itoa(ospfInterface["retransmit_interval"].(int)))
		}
		if ospfInterface["dead_interval"].(int) != 0 {
			configSet = append(configSet, setPrefixInterface+"dead-interval "+
				strconv.Itoa(ospfInterface["dead_interval"].(int)))
		}
	}
	if err := sess.configSet(configSet, jnprSess); err != nil {
		return err
	}

	return nil
}
func readOspfArea(idArea, version, routingInstance string,
	m interface{}, jnprSess *NetconfObject) (ospfAreaOptions, error) {
	sess := m.(*Session)
	var confRead ospfAreaOptions
	var ospfAreaConfig string
	var err error
	ospfVersion := opsfV2
	if version == "v3" {
		ospfVersion = ospfV3
	}
	if routingInstance == defaultWord {
		ospfAreaConfig, err = sess.command("show configuration protocols "+
			ospfVersion+" area "+idArea+" | display set relative", jnprSess)
		if err != nil {
			return confRead, err
		}
	} else {
		ospfAreaConfig, err = sess.command("show configuration routing-instances "+
			routingInstance+" protocols "+ospfVersion+" area "+idArea+" | display set relative", jnprSess)
		if err != nil {
			return confRead, err
		}
	}

	if ospfAreaConfig != emptyWord {
		confRead.areaID = idArea
		confRead.version = version
		confRead.routingInstance = routingInstance
		for _, item := range strings.Split(ospfAreaConfig, "\n") {
			if strings.Contains(item, "<configuration-output>") {
				continue
			}
			if strings.Contains(item, "</configuration-output>") {
				break
			}
			itemTrim := strings.TrimPrefix(item, setLineStart)
			if strings.HasPrefix(itemTrim, "interface ") {
				itemInterfaceList := strings.Split(strings.TrimPrefix(itemTrim, "interface "), " ")
				interfaceOptions := map[string]interface{}{
					"name":                itemInterfaceList[0],
					"disable":             false,
					"passive":             false,
					"metric":              0,
					"hello_interval":      0,
					"retransmit_interval": 0,
					"dead_interval":       0,
				}
				itemTrimInterface := strings.TrimPrefix(itemTrim, "interface "+itemInterfaceList[0]+" ")
				interfaceOptions, confRead.interFace = copyAndRemoveItemMapList("name", false, interfaceOptions, confRead.interFace)
				switch {
				case strings.HasPrefix(itemTrimInterface, "disable"):
					interfaceOptions["disable"] = true
				case strings.HasPrefix(itemTrimInterface, "passive"):
					interfaceOptions["passive"] = true
				case strings.HasPrefix(itemTrimInterface, "metric "):
					interfaceOptions["metric"], err = strconv.Atoi(
						strings.TrimPrefix(itemTrimInterface, "metric "))
					if err != nil {
						return confRead, fmt.Errorf("failed to convert value from '%s' to integer : %w", itemTrimInterface, err)
					}
				case strings.HasPrefix(itemTrimInterface, "hello-interval "):
					interfaceOptions["hello_interval"], err = strconv.Atoi(
						strings.TrimPrefix(itemTrimInterface, "hello-interval "))
					if err != nil {
						return confRead, fmt.Errorf("failed to convert value from '%s' to integer : %w", itemTrimInterface, err)
					}
				case strings.HasPrefix(itemTrimInterface, "retransmit-interval "):
					interfaceOptions["retransmit_interval"], err = strconv.Atoi(
						strings.TrimPrefix(itemTrimInterface, "retransmit-interval "))
					if err != nil {
						return confRead, fmt.Errorf("failed to convert value from '%s' to integer : %w", itemTrimInterface, err)
					}
				case strings.HasPrefix(itemTrimInterface, "dead-interval "):
					interfaceOptions["dead_interval"], err = strconv.Atoi(
						strings.TrimPrefix(itemTrimInterface, "dead-interval "))
					if err != nil {
						return confRead, fmt.Errorf("failed to convert value from '%s' to integer : %w", itemTrimInterface, err)
					}
				}
				confRead.interFace = append(confRead.interFace, interfaceOptions)
			}
		}
	} else {
		confRead.areaID = ""

		return confRead, nil
	}

	return confRead, nil
}

func delOspfArea(d *schema.ResourceData, m interface{}, jnprSess *NetconfObject) error {
	sess := m.(*Session)
	configSet := make([]string, 0, 1)
	ospfVersion := opsfV2
	if d.Get("version").(string) == "v3" {
		ospfVersion = ospfV3
	}
	if d.Get("routing_instance").(string) == defaultWord {
		configSet = append(configSet, "delete protocols "+ospfVersion+" area "+d.Get("area_id").(string))
	} else {
		configSet = append(configSet, "delete routing-instances "+d.Get("routing_instance").(string)+
			" protocols "+ospfVersion+" area "+d.Get("area_id").(string))
	}
	if err := sess.configSet(configSet, jnprSess); err != nil {
		return err
	}

	return nil
}

func fillOspfAreaData(d *schema.ResourceData, ospfAreaOptions ospfAreaOptions) {
	if tfErr := d.Set("area_id", ospfAreaOptions.areaID); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("routing_instance", ospfAreaOptions.routingInstance); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("version", ospfAreaOptions.version); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("interface", ospfAreaOptions.interFace); tfErr != nil {
		panic(tfErr)
	}
}
