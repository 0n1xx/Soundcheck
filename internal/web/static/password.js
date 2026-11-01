// Adds an eye button to every password field, to show or hide what was typed.
// Without JavaScript the fields still work as normal password fields.
(function () {
    const eye =
        '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" stroke-width="1.6"/></svg>';
    const eyeOff =
        '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M4 4l16 16" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>';

    document.querySelectorAll('input[type="password"]').forEach(function (input) {
        // Wrap the field so the button can sit inside it, on the right.
        const wrapper = document.createElement("span");
        wrapper.className = "password-field";
        input.parentNode.insertBefore(wrapper, input);
        wrapper.appendChild(input);

        const button = document.createElement("button");
        button.type = "button"; // a plain button, so it never submits the form
        button.className = "password-toggle";
        wrapper.appendChild(button);

        function setShown(shown) {
            input.type = shown ? "text" : "password";
            button.innerHTML = shown ? eyeOff : eye;
            button.setAttribute("aria-label", shown ? "Hide password" : "Show password");
            button.setAttribute("aria-pressed", String(shown));
        }
        setShown(false);

        button.addEventListener("click", function () {
            setShown(input.type === "password");
            input.focus();
        });

        // Hide it again before sending, so the browser's password manager
        // still sees a password field.
        if (input.form) {
            input.form.addEventListener("submit", function () {
                setShown(false);
            });
        }
    });
})();
