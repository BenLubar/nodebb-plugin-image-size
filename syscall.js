var libsys = require('libsys');

module.exports = {
	"Syscall": function() {
		var ret = libsys.syscall.apply(libsys, arguments);
		if (ret < 0) {
			return [-1, 0, -ret];
		}
		return [ret, 0, 0];
	},
	"Syscall6": function() {
		var ret = libsys.syscall64.apply(libsys, arguments);
		return [ret[0], 0, ret[1]];
	}
};
