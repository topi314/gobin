document.querySelectorAll(".styles li a").forEach((style) => {
    style.addEventListener("click", (event) => {
        event.preventDefault();

        const current = document.querySelector(".style .current");
        const currentStyle = current.textContent;
        const nextStyle = event.target.textContent;

        if (currentStyle !== nextStyle) {
            document
                .querySelector(`link[title="${nextStyle}"]`)
                .removeAttribute("disabled");
            document
                .querySelector(`link[title="${currentStyle}"]`)
                .setAttribute("disabled", "disabled");

            current.classList.remove("current");
            event.target.classList.add("current");
        }
    });
});
