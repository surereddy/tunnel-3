package proxy

type UserPass map[string]string

func NewUserPass(passes map[string]string) UserPass {
	if passes == nil {
		return make(UserPass)
	}
	return passes
}

func (up UserPass) Add(user, pass string) {
	up[user] = pass
}

func (up UserPass) Del(user string) {
	delete(up, user)
}

func (up UserPass) Has(user string) bool {
	_, has := up[user]
	return has
}

func (up UserPass) Verify(user, pass string) bool {
	p, has := up[user]
	return has && p == pass
}

func (up UserPass) One() (user, pass string, has bool) {
	for user, pass = range up {
		return user, pass, true
	}
	return "", "", false
}
