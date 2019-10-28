package ram

import (
	"github.com/eudore/eudore"
	"strconv"
	"strings"
)

type (
	// Pbac 定义PBAC鉴权对象。
	Pbac struct {
		PolicyBinds map[int][]int   `json:"-" key:"-"`
		Policys     map[int]*Policy `json:"-" key:"-"`
		GetResource func(eudore.Context) string
	}
)

// NewPbac 函数创建一个*Pbac对象
func NewPbac() *Pbac {
	return &Pbac{
		PolicyBinds: make(map[int][]int),
		Policys:     make(map[int]*Policy),
		GetResource: getResource,
	}
}

func getResource(ctx eudore.Context) string {
	path := ctx.Path()
	// 移除无效的前缀
	prefix := ctx.GetParam("prefix")
	if prefix != "" {
		path = path[len(prefix):]
	}
	ctx.SetParam("resource", path)
	return path
}

// BindPolicy 方法给一个用户id绑定一个策略id
func (p *Pbac) BindPolicy(id int, policyid int) {
	p.PolicyBinds[id] = append(p.PolicyBinds[id], policyid)
}

// BindStrings 方法给一个用户id绑定多个策略字符串id，使用逗号未分隔符
//
// p.BindStrings(0, "0,1,2")
func (p *Pbac) BindStrings(id int, str string) {
	strs := strings.Split(str, ",")
	var ps []int = make([]int, len(strs))
	for i, id := range strs {
		ps[i], _ = strconv.Atoi(id)
	}
	p.PolicyBinds[id] = append(p.PolicyBinds[id], ps...)
}

// AddPolicy 给PBAC添加一个策略。
func (p *Pbac) AddPolicy(id int, policy *Policy) {
	p.Policys[id] = policy
}

// AddPolicyStringJson 给PBAC添加一个策略，策略类型是JSON字符串。
func (p *Pbac) AddPolicyStringJson(id int, str string) {
	p.AddPolicy(id, NewPolicyStringJSON(str))
}

// Match 方法实现ram.Handler接口，匹配一个请求。
func (p *Pbac) Match(id int, action string, ctx eudore.Context) (bool, bool) {
	resource := p.GetResource(ctx)
	bs, ok := p.PolicyBinds[id]
	if ok {
		for _, b := range bs {
			ps, ok := p.Policys[b]
			if !ok {
				continue
			}
			for _, s := range ps.Statement {
				if s.MatchAction(action) && s.MatchResource(resource) && s.MatchCondition(ctx) {
					ctx.SetParam(eudore.ParamRAM, "pbac")
					return s.Effect, true
				}
			}
		}
	}
	return false, false
}
