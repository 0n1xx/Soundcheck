// Mobile menu: on narrow screens the sidebar slides in from the left when
// the ☰ button in the toolbar is pressed, like a drawer.
//
// This file is loaded in <head>, so the "has-nav" class is set before the
// page is drawn and the sidebar never flashes on screen. Without JavaScript
// the class is missing and the CSS keeps a simple scrollable bar instead.
document.documentElement.classList.add("has-nav");

document.addEventListener("DOMContentLoaded", function () {
    const root = document.documentElement;
    const button = document.querySelector(".menu-button");
    const sidebar = document.getElementById("sidebar");
    if (!button || !sidebar) {
        return;
    }

    // A dark layer behind the open menu; tapping it closes the menu.
    const backdrop = document.createElement("div");
    backdrop.className = "nav-backdrop";
    document.body.appendChild(backdrop);

    function setOpen(open) {
        root.classList.toggle("nav-open", open);
        button.setAttribute("aria-expanded", String(open));
        button.setAttribute("aria-label", open ? "Close menu" : "Open menu");
    }

    button.addEventListener("click", function () {
        setOpen(!root.classList.contains("nav-open"));
    });
    backdrop.addEventListener("click", function () {
        setOpen(false);
    });
    document.addEventListener("keydown", function (event) {
        if (event.key === "Escape") {
            setOpen(false);
        }
    });
});
