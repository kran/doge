(function(){
  function Validator(data) {
    this.reset();
    this._data = data || {};
  }

  Validator.Error = function(errors) {
    this._errors = errors;
  }

  Validator.Error.prototype = {
    constructor: Validator.Error,

    errors: function() {
      return this._errors;
    }
  }

  Validator.prototype = {
    constructor: Validator,

    reset: function() {
      this._data = {};
      this._errors = {};
      this._field = '';
      this._historyFields = [];
      this._attrs = {};
    },

    data: function(data) {
      this._data = data;

      return this;
    },

    field: function(field, attrName) {
      this._field = field;
      this._historyFields.push(field);

      if(attrName) this._attrs[field] = attrName;

      return this;
    },

    value: function() {
      return _.get(this._data, this._field);
    },

    removeValue: function() {
      var obj = this._data;
      var path = _.toPath(this._field);
      let idx = 0;

      for(idx = 0; idx < path.length - 1; idx++) {
        if(!_.has(obj, path[idx]))  {
          obj[path[idx]] = {};
        }

        obj = obj[path[idx]];
      }

      delete obj[path[idx]];
    }, 

    setValue: function(value) {
      var obj = this._data;
      var path = _.toPath(this._field);
      let idx = 0;

      for(idx = 0; idx < path.length - 1; idx++) {
        if(!_.has(obj, path[idx]))  {
          obj[path[idx]] = {};
        }

        obj = obj[path[idx]];
      }

      obj[path[idx]] = value;
    },

    setError: function(msg) {
      this._errors[this._field] = msg;
    },

    isExist: function() {
      return this.value() !== undefined;
    },

    isHasError: function() {
      return this._errors[this._field] !== undefined;
    },

    isEmpty: function() {
      let value = this.value();
      return !value || ( _.isArray(value) && value.length === 0 );
    },

    skip: function() {
      return this.isHasError() || !this.isExist();
    },

    exist: function() {
      if(this.isHasError()) {
        return this;
      }

      if(!this.isExist()) {
        this.setError(`参数{${this._field}}必须存在`);
      }

      return this;
    },

    notEmpty: function() {
      if(this.skip()) {
        return this;
      }

      if(this.isEmpty()) {
        this.setError(`{${this._field}}不能为空`);
      }

      return this;
    },

    min: function(n) {
      if(this.skip()) {
        return this;
      }

      if(this.value() < n) {
        this.setError(`{${this._field}}必须大于${n}`);
      }

      return this;
    },

    max: function(n) {
      if(this.skip()) {
        return this;
      }

      if(this.value() > n) {
        this.setError(`{${this._field}}必须小于${n}`);
      }

      return this;
    },

    array: function() {
      if(this.skip()) {
        return this;
      }

      if(!_.isArray(this.value())) {
        this.setError(`参数{${this._field}}必须为数组`);
      }

      return this;
    },

    in: function() {
      if(this.skip()) {
        return this;
      }

      var range = _.flatten(arguments);

      if(range.indexOf(this.value()) === -1) {
        this.setError(`{${this._field}}必须是${range.join(",")}之一`);
      }

      return this;
    },

    regexp: function(exp) {
      if(this.string().skip()) {
        return this;
      }

      if(!exp.test(this.value())) {
        this.setError(`参数{${this._field}}必须匹配${exp}`);
      }

      return this;
    },

    string: function() {
      if(this.skip()) {
        return this;
      }

      if(!_.isString(this.value())) {
        this.setError(`参数{${this._field}}必须为字符串`);
      }

      return this;
    },

    trim: function() {
      if(this.string().skip()) {
        return this;
      }

      this.setValue(this.value().trim());

      return this;
    },

    tap: function(fn) {
      if(this.skip()) {
        return this;
      }

      this.setValue(fn(this.value()));

      return this;
    },

    commaArray: function() {
      if(this.string().skip()) {
        return this;
      }

      this.setValue(_.compact(this.value().split(',')));

      return this;
    },

    unique: function() {
      if(this.array().skip()) {
        return this;
      }

      let value = _.unique(this.value());
      this.setValue(value);

      return this;
    },

    toUpper: function() {
      if(this.string().skip()) {
        return this;
      }

      let value = this.value().toUpperCase();
      this.setValue(value);

      return this;
    },

    removeEmpty: function() {
      if(this.skip()) {
        return this;
      }

      if(this.isEmpty()) {
        this.removeValue();
      }

      return this;
    },

    defaultValue: function(value) {
      if(this.isEmpty()) {
        this.setValue(value);
      }

      return this;
    },

    integer: function() {
      if(this.skip()) {
        return this;
      }

      this.setValue(parseInt(this.value()));

      if(_.isNaN(this.value())) {
        this.setError(`{${this._field}}必须为数字`);
      }

      return this;
    },

    toTimestamp: function() {
      if(this.skip()) {
        return this;
      }

      var date = Date.parse(this.value());
      if(_.isNaN(date)) {
        this.setError(`${this._field}必须为有效的日期时间格式`);
      }
      else {
        this.setValue(date);
      }

      return this;
    },

    must: function(fn, err) {
      if(this.skip()) {
        return this;
      }

      if(!fn(this.value())) {
        this.setError(err || `{${this._field}}未满足条件`);
      }

      return this;
    },

    validate: function(pickValidatedFields) {
      if(_.size(this._errors) > 0) {

        for(var attr in this._errors) {
          if(this._attrs[attr]) {
            this._errors[attr] = this._errors[attr].replace(`{${attr}}`, this._attrs[attr]);
          }
        }

        throw new Validator.Error(this._errors);
      }

      let handledData = pickValidatedFields 
        ? _.pick(this._data, this._historyFields)
        : this._data;

      this.reset();

      return handledData;
    },
  };

  return {
    create: function(data) {
      return new Validator(data);
    },
    Error: Validator.Error,
  }
})();
