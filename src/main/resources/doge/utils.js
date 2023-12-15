Function.prototype.extends = function(parent) {
  this.__proto__ = Object.create(parent);
  this.prototype.__proto__ = Object.create(parent.prototype);
  this.prototype.constructor = this;

  this.$super = function(self) {
    return {
      constructor: function(){
        parent.apply(self, arguments);
      },

      __noSuchMethod__: function(name) {
        return parent.prototype[name].apply(self, _.rest(arguments));
      },

      __noSuchProperty__: function(name) {
        return parent.prototype[name];
      }
    }
  }

  this.prototype.$class = function() {
    return this.constructor;
  }

  return this;
}
