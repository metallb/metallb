// https://github.com/inexorabletash/polyfill/blob/v0.1.25/url.js

// URL Polyfill
// Draft specification: https://url.spec.whatwg.org

// Notes:
// - Primarily useful for parsing URLs and modifying query parameters
// - Should work in IE8+ and everything more modern

(function (global) {
	'use strict';

	// Browsers may have:
	// * No global URL object
	// * URL with static methods only - may have a dummy constructor
	// * URL with members except searchParams
	// * Full URL API support
	var origURL = global.URL;
	var nativeURL;
	try{
		if( origURL ){
			nativeURL = new global.URL('http://example.com');
			if( 'searchParams' in nativeURL )
				return;
			if( !('href' in nativeURL) )
				nativeURL = undefined;
		}
	}
	catch( _ ){}

	// NOTE: Doesn't do the encoding/decoding dance
	function urlencoded_serialize(pairs) {
		var output = '', first = true;
		pairs.forEach(function (pair) {
			var name = encodeURIComponent(pair.name);
			var value = encodeURIComponent(pair.value);
			if( !first ) output += '&';
			output += name + '=' + value;
			first = false;
		});
		return output.replace(/%20/g, '+');
	}

	// NOTE: Doesn't do the encoding/decoding dance
	function urlencoded_parse(input, isindex) {
		var sequences = input.split('&');
		if( isindex && sequences[0].indexOf('=') === -1 )
			sequences[0] = '=' + sequences[0];
		var pairs = [];
		sequences.forEach(function (bytes) {
			if( bytes.length === 0 ) return;
			var index = bytes.indexOf('=');
			if( index !== -1 ){
				var name = bytes.substring(0, index);
				var value = bytes.substring(index + 1);
			}
			else{
				name = bytes;
				value = '';
			}
			name = name.replace(/\+/g, ' ');
			value = value.replace(/\+/g, ' ');
			pairs.push({ name: name, value: value });
		});
		var output = [];
		pairs.forEach(function (pair) {
			output.push({
				name: decodeURIComponent(pair.name),
				value: decodeURIComponent(pair.value)
			});
		});
		return output;
	}

	function URLUtils(url) {
		if( nativeURL )
			return new origURL(url);
		var anchor = document.createElement('a');
		anchor.href = url;
		return anchor;
	}

	function URLSearchParams(init) {
		var $this = this;
		this._list = [];

		if( init === undefined || init === null )
			init = '';

		if( Object(init) !== init || !(init instanceof URLSearchParams) )
			init = String(init);

		if( typeof init === 'string' && init.substring(0, 1) === '?' )
			init = init.substring(1);

		if( typeof init === 'string' )
			this._list = urlencoded_parse(init);
		else
			this._list = init._list.slice();

		this._url_object = null;
		this._setList = function (list) { if( !updating ) $this._list = list; };

		var updating = false;
		this._update_steps = function() {
			if( updating ) return;
			updating = true;

			if( !$this._url_object ) return;

			// Partial workaround for IE issue with 'about:'
			if( $this._url_object.protocol === 'about:' &&
				$this._url_object.pathname.indexOf('?') !== -1 ){
				$this._url_object.pathname = $this._url_object.pathname.split('?')[0];
			}

			$this._url_object.search = urlencoded_serialize($this._list);

			updating = false;
		};
	}

	Object.defineProperties(URLSearchParams.prototype, {
		append: {
			value: function (name, value) {
				this._list.push({ name: name, value: value });
				this._update_steps();
			}, writable: true, enumerable: true, configurable: true
		},

		'delete': {
			value: function (name) {
				for( var i = 0; i < this._list.length; ){
					if( this._list[i].name === name )
						this._list.splice(i, 1);
					else
						++i;
				}
				this._update_steps();
			}, writable: true, enumerable: true, configurable: true
		},

		get: {
			value: function (name) {
				for( var i = 0; i < this._list.length; ++i ){
					if( this._list[i].name === name )
						return this._list[i].value;
				}
				return null;
			}, writable: true, enumerable: true, configurable: true
		},

		getAll: {
			value: function (name) {
				var result = [];
				for( var i = 0; i < this._list.length; ++i ){
					if( this._list[i].name === name )
						result.push(this._list[i].value);
				}
				return result;
			}, writable: true, enumerable: true, configurable: true
		},

		has: {
			value: function (name) {
				for( var i = 0; i < this._list.length; ++i ){
					if( this._list[i].name === name )
						return true;
				}
				return false;
			}, writable: true, enumerable: true, configurable: true
		},

		set: {
			value: function (name, value) {
				var found = false;
				for( var i = 0; i < this._list.length; ){
					if( this._list[i].name === name ){
						if( !found ){
							this._list[i].value = value;
							found = true;
							++i;
						}
						else{
							this._list.splice(i, 1);
						}
					}
					else{
						++i;
					}
				}

				if( !found )
					this._list.push({ name: name, value: value });

				this._update_steps();
			}, writable: true, enumerable: true, configurable: true
		},

		entries: {
			value: function() {
				var $this = this, index = 0;
				return { next: function() {
					if( index >= $this._list.length )
						return {done: true, value: undefined};
					var pair = $this._list[index++];
					return {done: false, value: [pair.name, pair.value]};
				}};
			}, writable: true, enumerable: true, configurable: true
		},

		keys: {
			value: function() {
				var $this = this, index = 0;
				return { next: function() {
					if( index >= $this._list.length )
						return {done: true, value: undefined};
					var pair = $this._list[index++];
					return {done: false, value: pair.name};
				}};
			}, writable: true, enumerable: true, configurable: true
		},

		values: {
			value: function() {
				var $this = this, index = 0;
				return { next: function() {
					if( index >= $this._list.length )
						return {done: true, value: undefined};
					var pair = $this._list[index++];
					return {done: false, value: pair.value};
				}};
			}, writable: true, enumerable: true, configurable: true
		},

		forEach: {
			value: function(callback) {
				var thisArg = (arguments.length > 1) ? arguments[1] : undefined;
				this._list.forEach(function(pair, _index) {
					callback.call(thisArg, pair.name, pair.value);
				});

			}, writable: true, enumerable: true, configurable: true
		},

		toString: {
			value: function () {
				return urlencoded_serialize(this._list);
			}, writable: true, enumerable: false, configurable: true
		}
	});

	if( 'Symbol' in global && 'iterator' in global.Symbol ){
		Object.defineProperty(URLSearchParams.prototype, global.Symbol.iterator, {
			value: URLSearchParams.prototype.entries,
			writable: true, enumerable: true, configurable: true});
	}

	function URL(url, base) {
		if( !(this instanceof global.URL) )
			throw new TypeError('Failed to construct "URL": Please use the "new" operator.');

		if( base ){
			url = (function () {
				if( nativeURL ) return new origURL(url, base).href;

				var doc;
				// Use another document/base tag/anchor for relative URL resolution, if possible
				if( document.implementation && document.implementation.createHTMLDocument ){
					doc = document.implementation.createHTMLDocument('');
				}
				else if( document.implementation && document.implementation.createDocument ){
					doc = document.implementation.createDocument('http://www.w3.org/1999/xhtml', 'html', null);
					doc.documentElement.appendChild(doc.createElement('head'));
					doc.documentElement.appendChild(doc.createElement('body'));
				}
				else if( window.ActiveXObject ){
					doc = new window.ActiveXObject('htmlfile');
					doc.write('<head></head><body></body>');
					doc.close();
				}

				if( !doc ) throw Error('base not supported');

				var baseTag = doc.createElement('base');
				baseTag.href = base;
				doc.getElementsByTagName('head')[0].appendChild(baseTag);
				var anchor = doc.createElement('a');
				anchor.href = url;
				return anchor.href;
			}());
		}

		// An inner object implementing URLUtils (either a native URL
		// object or an HTMLAnchorElement instance) is used to perform the
		// URL algorithms. With full ES5 getter/setter support, return a
		// regular object For IE8's limited getter/setter support, a
		// different HTMLAnchorElement is returned with properties
		// overridden

		var instance = URLUtils(url || '');

		// Detect for ES5 getter/setter support
		// (an Object.defineProperties polyfill that doesn't support getters/setters may throw)
		var ES5_GET_SET = (function() {
			if( !('defineProperties' in Object) ) return false;
			try{
				var obj = {};
				Object.defineProperties(obj, { prop: { 'get': function () { return true; } } });
				return obj.prop;
			}
			catch( _ ){
				return false;
			}
		})();

		var self = ES5_GET_SET ? this : document.createElement('a');

		var query_object = new URLSearchParams(
			instance.search ? instance.search.substring(1) : null);
		query_object._url_object = self;

		Object.defineProperties(self, {
			href: {
				get: function () { return instance.href; },
				set: function (v) { instance.href = v; tidy_instance(); update_steps(); },
				enumerable: true, configurable: true
			},
			origin: {
				get: function () {
					if( 'origin' in instance ) return instance.origin;
					return this.protocol + '//' + this.host;
				},
				enumerable: true, configurable: true
			},
			protocol: {
				get: function () { return instance.protocol; },
				set: function (v) { instance.protocol = v; },
				enumerable: true, configurable: true
			},
			username: {
				get: function () { return instance.username; },
				set: function (v) { instance.username = v; },
				enumerable: true, configurable: true
			},
			password: {
				get: function () { return instance.password; },
				set: function (v) { instance.password = v; },
				enumerable: true, configurable: true
			},
			host: {
				get: function () {
					// IE returns default port in |host|
					var re = {'http:': /:80$/, 'https:': /:443$/, 'ftp:': /:21$/}[instance.protocol];
					return re ? instance.host.replace(re, '') : instance.host;
				},
				set: function (v) { instance.host = v; },
				enumerable: true, configurable: true
			},
			hostname: {
				get: function () { return instance.hostname; },
				set: function (v) { instance.hostname = v; },
				enumerable: true, configurable: true
			},
			port: {
				get: function () { return instance.port; },
				set: function (v) { instance.port = v; },
				enumerable: true, configurable: true
			},
			pathname: {
				get: function () {
					// IE does not include leading '/' in |pathname|
					if( instance.pathname.charAt(0) !== '/' ) return '/' + instance.pathname;
					return instance.pathname;
				},
				set: function (v) { instance.pathname = v; },
				enumerable: true, configurable: true
			},
			search: {
				get: function () { return instance.search; },
				set: function (v) {
					if( instance.search === v ) return;
					instance.search = v; tidy_instance(); update_steps();
				},
				enumerable: true, configurable: true
			},
			searchParams: {
				get: function () { return query_object; },
				enumerable: true, configurable: true
			},
			hash: {
				get: function () { return instance.hash; },
				set: function (v) { instance.hash = v; tidy_instance(); },
				enumerable: true, configurable: true
			},
			toString: {
				value: function() { return instance.toString(); },
				enumerable: false, configurable: true
			},
			valueOf: {
				value: function() { return instance.valueOf(); },
				enumerable: false, configurable: true
			}
		});

		function tidy_instance() {
			var href = instance.href.replace(/#$|\?$|\?(?=#)/g, '');
			if( instance.href !== href )
				instance.href = href;
		}

		function update_steps() {
			query_object._setList(instance.search ? urlencoded_parse(instance.search.substring(1)) : []);
			query_object._update_steps();
		}

		return self;
	}

	if( origURL ){
		for( var i in origURL ){
			if( Object.prototype.hasOwnProperty.call(origURL, i) && typeof origURL[i] === 'function' )
				URL[i] = origURL[i];
		}
	}

	global.URL = URL;
	global.URLSearchParams = URLSearchParams;

}( typeof self !== 'undefined' ? self : this ));
