package junos

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ipsecPolicyOptions struct {
	name      string
	pfsKeys   string
	proposals []string
}

func resourceIpsecPolicy() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIpsecPolicyCreate,
		ReadContext:   resourceIpsecPolicyRead,
		UpdateContext: resourceIpsecPolicyUpdate,
		DeleteContext: resourceIpsecPolicyDelete,
		Importer: &schema.ResourceImporter{
			State: resourceIpsecPolicyImport,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:             schema.TypeString,
				ForceNew:         true,
				Required:         true,
				ValidateDiagFunc: validateNameObjectJunos([]string{}),
			},
			"proposals": {
				Type:     schema.TypeList,
				Required: true,
				MinItems: 1,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"pfs_keys": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceIpsecPolicyCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	if !checkCompatibilitySecurity(jnprSess) {
		return diag.FromErr(fmt.Errorf("security ipsec policy not compatible with Junos device %s",
			jnprSess.Platform[0].Model))
	}
	sess.configLock(jnprSess)
	ipsecPolicyExists, err := checkIpsecPolicyExists(d.Get("name").(string), m, jnprSess)
	if err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if ipsecPolicyExists {
		sess.configClear(jnprSess)

		return diag.FromErr(fmt.Errorf("security ipsec policy %v already exists", d.Get("name").(string)))
	}
	if err := setIpsecPolicy(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("create resource junos_security_ipsec_policy", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	ipsecPolicyExists, err = checkIpsecPolicyExists(d.Get("name").(string), m, jnprSess)
	if err != nil {
		return diag.FromErr(err)
	}
	if ipsecPolicyExists {
		d.SetId(d.Get("name").(string))
	} else {
		return diag.FromErr(fmt.Errorf("security ipsec policy %v not exists after commit "+
			"=> check your config", d.Get("name").(string)))
	}

	return resourceIpsecPolicyRead(ctx, d, m)
}
func resourceIpsecPolicyRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	mutex.Lock()
	jnprSess, err := sess.startNewSession()
	if err != nil {
		mutex.Unlock()

		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	ipsecPolicyOptions, err := readIpsecPolicy(d.Get("name").(string), m, jnprSess)
	mutex.Unlock()
	if err != nil {
		return diag.FromErr(err)
	}
	if ipsecPolicyOptions.name == "" {
		d.SetId("")
	} else {
		fillIpsecPolicyData(d, ipsecPolicyOptions)
	}

	return nil
}
func resourceIpsecPolicyUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	d.Partial(true)
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	sess.configLock(jnprSess)
	if err := delIpsecPolicy(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := setIpsecPolicy(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("update resource junos_security_ipsec_policy", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	d.Partial(false)

	return resourceIpsecPolicyRead(ctx, d, m)
}
func resourceIpsecPolicyDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	sess.configLock(jnprSess)
	if err := delIpsecPolicy(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("delete resource junos_security_ipsec_policy", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}

	return nil
}
func resourceIpsecPolicyImport(d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return nil, err
	}
	defer sess.closeSession(jnprSess)
	result := make([]*schema.ResourceData, 1)
	ipsecPolicyExists, err := checkIpsecPolicyExists(d.Id(), m, jnprSess)
	if err != nil {
		return nil, err
	}
	if !ipsecPolicyExists {
		return nil, fmt.Errorf("don't find security ipsec policy with id '%v' (id must be <name>)", d.Id())
	}
	ipsecPolicyOptions, err := readIpsecPolicy(d.Id(), m, jnprSess)
	if err != nil {
		return nil, err
	}
	fillIpsecPolicyData(d, ipsecPolicyOptions)
	result[0] = d

	return result, nil
}

func checkIpsecPolicyExists(ipsecPolicy string, m interface{}, jnprSess *NetconfObject) (bool, error) {
	sess := m.(*Session)
	ipsecPolicyConfig, err := sess.command("show configuration"+
		" security ipsec policy "+ipsecPolicy+" | display set", jnprSess)
	if err != nil {
		return false, err
	}
	if ipsecPolicyConfig == emptyWord {
		return false, nil
	}

	return true, nil
}
func setIpsecPolicy(d *schema.ResourceData, m interface{}, jnprSess *NetconfObject) error {
	sess := m.(*Session)
	configSet := make([]string, 0)

	setPrefix := "set security ipsec policy " + d.Get("name").(string)
	if d.Get("pfs_keys").(string) != "" {
		configSet = append(configSet, setPrefix+" perfect-forward-secrecy keys "+d.Get("pfs_keys").(string))
	}
	for _, v := range d.Get("proposals").([]interface{}) {
		configSet = append(configSet, setPrefix+" proposals "+v.(string))
	}

	if err := sess.configSet(configSet, jnprSess); err != nil {
		return err
	}

	return nil
}
func readIpsecPolicy(ipsecPolicy string, m interface{}, jnprSess *NetconfObject) (ipsecPolicyOptions, error) {
	sess := m.(*Session)
	var confRead ipsecPolicyOptions

	ipsecPolicyConfig, err := sess.command("show configuration"+
		" security ipsec policy "+ipsecPolicy+" | display set relative", jnprSess)
	if err != nil {
		return confRead, err
	}
	if ipsecPolicyConfig != emptyWord {
		confRead.name = ipsecPolicy
		for _, item := range strings.Split(ipsecPolicyConfig, "\n") {
			if strings.Contains(item, "<configuration-output>") {
				continue
			}
			if strings.Contains(item, "</configuration-output>") {
				break
			}
			itemTrim := strings.TrimPrefix(item, setLineStart)
			switch {
			case strings.HasPrefix(itemTrim, "proposals "):
				confRead.proposals = append(confRead.proposals, strings.TrimPrefix(itemTrim, "proposals "))
			case strings.HasPrefix(itemTrim, "perfect-forward-secrecy keys "):
				confRead.pfsKeys = strings.TrimPrefix(itemTrim, "perfect-forward-secrecy keys ")
			}
		}
	} else {
		confRead.name = ""

		return confRead, nil
	}

	return confRead, nil
}
func delIpsecPolicy(d *schema.ResourceData, m interface{}, jnprSess *NetconfObject) error {
	sess := m.(*Session)
	configSet := make([]string, 0, 1)
	configSet = append(configSet, "delete security ipsec policy "+d.Get("name").(string))
	if err := sess.configSet(configSet, jnprSess); err != nil {
		return err
	}

	return nil
}

func fillIpsecPolicyData(d *schema.ResourceData, ipsecPolicyOptions ipsecPolicyOptions) {
	if tfErr := d.Set("name", ipsecPolicyOptions.name); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("proposals", ipsecPolicyOptions.proposals); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("pfs_keys", ipsecPolicyOptions.pfsKeys); tfErr != nil {
		panic(tfErr)
	}
}
