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

type ikeProposalOptions struct {
	lifetimeSeconds         int
	name                    string
	authenticationAlgorithm string
	authenticationMethod    string
	dhGroup                 string
	encryptionAlgorithm     string
}

func resourceIkeProposal() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIkeProposalCreate,
		ReadContext:   resourceIkeProposalRead,
		UpdateContext: resourceIkeProposalUpdate,
		DeleteContext: resourceIkeProposalDelete,
		Importer: &schema.ResourceImporter{
			State: resourceIkeProposalImport,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:             schema.TypeString,
				ForceNew:         true,
				Required:         true,
				ValidateDiagFunc: validateNameObjectJunos([]string{}),
			},
			"authentication_algorithm": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"authentication_method": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "pre-shared-keys",
			},
			"dh_group": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"encryption_algorithm": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"lifetime_seconds": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntBetween(180, 86400),
			},
		},
	}
}

func resourceIkeProposalCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	if !checkCompatibilitySecurity(jnprSess) {
		return diag.FromErr(fmt.Errorf("security ike proposal not compatible with Junos device %s",
			jnprSess.Platform[0].Model))
	}
	sess.configLock(jnprSess)
	ikeProposalExists, err := checkIkeProposalExists(d.Get("name").(string), m, jnprSess)
	if err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if ikeProposalExists {
		sess.configClear(jnprSess)

		return diag.FromErr(fmt.Errorf("security ike proposal %v already exists", d.Get("name").(string)))
	}
	if err := setIkeProposal(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("create resource junos_security_ike_proposal", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	ikeProposalExists, err = checkIkeProposalExists(d.Get("name").(string), m, jnprSess)
	if err != nil {
		return diag.FromErr(err)
	}
	if ikeProposalExists {
		d.SetId(d.Get("name").(string))
	} else {
		return diag.FromErr(fmt.Errorf("security ike proposal %v not exists after commit "+
			"=> check your config", d.Get("name").(string)))
	}

	return resourceIkeProposalRead(ctx, d, m)
}
func resourceIkeProposalRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	mutex.Lock()
	jnprSess, err := sess.startNewSession()
	if err != nil {
		mutex.Unlock()

		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	ikeProposalOptions, err := readIkeProposal(d.Get("name").(string), m, jnprSess)
	mutex.Unlock()
	if err != nil {
		return diag.FromErr(err)
	}
	if ikeProposalOptions.name == "" {
		d.SetId("")
	} else {
		fillIkeProposalData(d, ikeProposalOptions)
	}

	return nil
}
func resourceIkeProposalUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	d.Partial(true)
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	sess.configLock(jnprSess)
	if err := delIkeProposal(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := setIkeProposal(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("update resource junos_security_ike_proposal", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	d.Partial(false)

	return resourceIkeProposalRead(ctx, d, m)
}
func resourceIkeProposalDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return diag.FromErr(err)
	}
	defer sess.closeSession(jnprSess)
	sess.configLock(jnprSess)
	if err := delIkeProposal(d, m, jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}
	if err := sess.commitConf("delete resource junos_security_ike_proposal", jnprSess); err != nil {
		sess.configClear(jnprSess)

		return diag.FromErr(err)
	}

	return nil
}
func resourceIkeProposalImport(d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	sess := m.(*Session)
	jnprSess, err := sess.startNewSession()
	if err != nil {
		return nil, err
	}
	defer sess.closeSession(jnprSess)
	result := make([]*schema.ResourceData, 1)
	ikeProposalExists, err := checkIkeProposalExists(d.Id(), m, jnprSess)
	if err != nil {
		return nil, err
	}
	if !ikeProposalExists {
		return nil, fmt.Errorf("don't find security ike proposal with id '%v' (id must be <name>)", d.Id())
	}
	ikeProposalOptions, err := readIkeProposal(d.Id(), m, jnprSess)
	if err != nil {
		return nil, err
	}
	fillIkeProposalData(d, ikeProposalOptions)
	result[0] = d

	return result, nil
}

func checkIkeProposalExists(ikeProposal string, m interface{}, jnprSess *NetconfObject) (bool, error) {
	sess := m.(*Session)
	ikeProposalConfig, err := sess.command("show configuration"+
		" security ike proposal "+ikeProposal+" | display set", jnprSess)
	if err != nil {
		return false, err
	}
	if ikeProposalConfig == emptyWord {
		return false, nil
	}

	return true, nil
}
func setIkeProposal(d *schema.ResourceData, m interface{}, jnprSess *NetconfObject) error {
	sess := m.(*Session)
	configSet := make([]string, 0)

	setPrefix := "set security ike proposal " + d.Get("name").(string)
	if d.Get("authentication_method").(string) != "" {
		configSet = append(configSet, setPrefix+" authentication-method "+d.Get("authentication_method").(string))
	}
	if d.Get("authentication_algorithm").(string) != "" {
		configSet = append(configSet, setPrefix+" authentication-algorithm "+d.Get("authentication_algorithm").(string))
	}
	if d.Get("dh_group").(string) != "" {
		configSet = append(configSet, setPrefix+" dh-group "+d.Get("dh_group").(string))
	}
	if d.Get("encryption_algorithm").(string) != "" {
		configSet = append(configSet, setPrefix+" encryption-algorithm "+d.Get("encryption_algorithm").(string))
	}
	if d.Get("lifetime_seconds").(int) != 0 {
		configSet = append(configSet, setPrefix+" lifetime-seconds "+strconv.Itoa(d.Get("lifetime_seconds").(int)))
	}

	if err := sess.configSet(configSet, jnprSess); err != nil {
		return err
	}

	return nil
}
func readIkeProposal(ikeProposal string, m interface{}, jnprSess *NetconfObject) (ikeProposalOptions, error) {
	sess := m.(*Session)
	var confRead ikeProposalOptions

	ikeProposalConfig, err := sess.command("show configuration"+
		" security ike proposal "+ikeProposal+" | display set relative", jnprSess)
	if err != nil {
		return confRead, err
	}
	if ikeProposalConfig != emptyWord {
		confRead.name = ikeProposal
		for _, item := range strings.Split(ikeProposalConfig, "\n") {
			if strings.Contains(item, "<configuration-output>") {
				continue
			}
			if strings.Contains(item, "</configuration-output>") {
				break
			}
			itemTrim := strings.TrimPrefix(item, setLineStart)
			switch {
			case strings.HasPrefix(itemTrim, "authentication-algorithm "):
				confRead.authenticationAlgorithm = strings.TrimPrefix(itemTrim, "authentication-algorithm ")
			case strings.HasPrefix(itemTrim, "authentication-method "):
				confRead.authenticationMethod = strings.TrimPrefix(itemTrim, "authentication-method ")
			case strings.HasPrefix(itemTrim, "dh-group "):
				confRead.dhGroup = strings.TrimPrefix(itemTrim, "dh-group ")
			case strings.HasPrefix(itemTrim, "encryption-algorithm"):
				confRead.encryptionAlgorithm = strings.TrimPrefix(itemTrim, "encryption-algorithm ")
			case strings.HasPrefix(itemTrim, "lifetime-seconds"):
				confRead.lifetimeSeconds, err = strconv.Atoi(strings.TrimPrefix(itemTrim, "lifetime-seconds "))
				if err != nil {
					return confRead, fmt.Errorf("failed to convert value from '%s' to integer : %w", itemTrim, err)
				}
			}
		}
	} else {
		confRead.name = ""

		return confRead, nil
	}

	return confRead, nil
}
func delIkeProposal(d *schema.ResourceData, m interface{}, jnprSess *NetconfObject) error {
	sess := m.(*Session)
	configSet := make([]string, 0, 1)
	configSet = append(configSet, "delete security ike proposal "+d.Get("name").(string))
	if err := sess.configSet(configSet, jnprSess); err != nil {
		return err
	}

	return nil
}

func fillIkeProposalData(d *schema.ResourceData, ikeProposalOptions ikeProposalOptions) {
	if tfErr := d.Set("name", ikeProposalOptions.name); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("authentication_algorithm", ikeProposalOptions.authenticationAlgorithm); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("authentication_method", ikeProposalOptions.authenticationMethod); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("dh_group", ikeProposalOptions.dhGroup); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("encryption_algorithm", ikeProposalOptions.encryptionAlgorithm); tfErr != nil {
		panic(tfErr)
	}
	if tfErr := d.Set("lifetime_seconds", ikeProposalOptions.lifetimeSeconds); tfErr != nil {
		panic(tfErr)
	}
}
