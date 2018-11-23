package container

func Contains(sl []interface{}, v interface{}) bool {
	for _, vv := range sl {
		if vv == v {
			return true
		}
	}
	return false
}

func ContainsInt(sl []int, v int) (bool, int) {
	for kk, vv := range sl {
		if vv == v {
			return true, kk
		}
	}
	return false, 0
}

func ContainsInt64(sl []int64, v int64) bool {
	for _, vv := range sl {
		if vv == v {
			return true
		}
	}
	return false
}

func ContainsString(sl []string, v string) bool {
	for _, vv := range sl {
		if vv == v {
			return true
		}
	}
	return false
}
