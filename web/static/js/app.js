// zelo custom client behaviour.
//
// Alpine.js is a ratified exception to constitution Principle VI, scoped to
// client-local interaction state only (see .specify/memory/constitution.md).
// Keep that scope: anything HTMX or Tailwind CSS can already do belongs there
// instead, not here.
//
// LOAD ORDER IS LOAD-BEARING: this file must be loaded BEFORE
// alpine-csp.min.js (see layout.templ). The CSP build starts Alpine from a
// microtask, and Alpine.start() dispatches `alpine:init` synchronously before
// it walks the DOM. Since deferred scripts are executed in document order,
// registering the listener here — while the Alpine script has not run yet — is
// what guarantees every component is registered before it is looked up.
//
// The directives in .templ files are deliberately limited to bare identifiers
// (`x-data="passwordToggle"`, `x-show="eyeVisible"`, `x-on:click="toggle"`,
// `x-bind:type="inputType"`). The CSP build's evaluator rejects `!x`, method
// arguments, arrow functions, template literals, and globals, so all logic
// lives in the components below, which are ordinary JavaScript.
document.addEventListener('alpine:init', function () {
	// Components live on Alpine's data registry rather than in inline scripts,
	// so they are registered once per page load no matter how often a template
	// is rendered or swapped in.
	Alpine.data('passwordToggle', function () {
		return {
			show: false,
			inputType: 'password',
			pressed: 'false',
			eyeVisible: true,
			eyeOffVisible: false,
			toggleLabel: '',
			showLabel: '',
			hideLabel: '',

			// Labels arrive as data-label-show / data-label-hide so they stay
			// translated (i18n) instead of being hardcoded in JavaScript.
			init: function () {
				this.showLabel = this.$el.dataset.labelShow;
				this.hideLabel = this.$el.dataset.labelHide;
				this.toggleLabel = this.showLabel;
			},

			// The reveal control needs no server round trip, which is why it is
			// client-local state here and not an HTMX request.
			toggle: function () {
				this.show = !this.show;
				this.inputType = this.show ? 'text' : 'password';
				this.pressed = this.show ? 'true' : 'false';
				this.eyeVisible = !this.show;
				this.eyeOffVisible = this.show;
				this.toggleLabel = this.show ? this.hideLabel : this.showLabel;
			},
		};
	});
});
