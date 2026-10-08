package engine

type DialogueLayout struct {
	Eligible bool `json:"eligible"`
	Choices  int  `json:"choices"`
}

func (p *Player) DialogueLayout() DialogueLayout {
	if p.Host == nil || !p.Ready {
		return DialogueLayout{}
	}
	d := p.Host.Director
	// Movies, snapshot fades and held screens keep their original composition.
	if !d.PuppetView.Wide || d.ScreenOwner() != "puppet" {
		return DialogueLayout{}
	}
	return DialogueLayout{Eligible: true, Choices: min(5, len(p.Host.Session.PuppetCtrl.Puppet.Bevels))}
}
