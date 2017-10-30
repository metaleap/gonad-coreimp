package 𝙜ˈDataˈEq

func RefEq(r1 interface{}) func(interface{}) bool {
	return func(r2 interface{}) bool {
		return r1 == r2
	}
}

func RefIneq(r1 interface{}) func(interface{}) bool {
	return func(r2 interface{}) bool {
		return r1 != r2
	}
}

func EqArrayImpl(f func(interface{}) func(interface{}) bool) func(interface{}) func(interface{}) bool {
	return func(xs interface{}) func(interface{}) bool {
		return func(ys interface{}) bool {
			sl1, sl2 := xs.([]interface{}), ys.([]interface{})
			if l := len(sl1); l != len(sl2) {
				return false
			} else {
				for i := 0; i < l; i++ {
					if !f(sl1[i])(sl2[i]) {
						return false
					}
				}
				return true
			}
		}
	}
}
