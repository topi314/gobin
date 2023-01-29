hljs.listLanguages().forEach((language) => {
    const option = document.createElement("option");
    option.value = language;
    option.innerText = language;
    document.querySelector("#language").appendChild(option);
});

document.addEventListener("DOMContentLoaded", () => {
    const key = window.location.pathname === "/" ? "" : window.location.pathname.slice(1);
    let newState;
    let url;
    if (key) {
        const content = document.querySelector("#code-show").innerText
        newState = {key: key, mode: "view", content: content};
        url = `/${key}`;
    } else {
        newState = {key: "", mode: "edit", content: ""};
        url = "/";
    }
    window.history.replaceState(newState, "", url);

    const style = localStorage.getItem("stylePreference") || "github-dark.min.css"
    setStyle(style);

    updatePage(newState);
    updatePageButtons(newState);
});

window.addEventListener("popstate", (event) => {
    updatePage(event.state);
    updatePageButtons(event.state);
});

document.querySelector("#code-edit").addEventListener("keyup", (event) => {
    const {key} = getState();
    const newState = {key: key, mode: "edit", content: event.target.value};
    window.history.replaceState(newState, "", `/${key}`);
    updatePageButtons(newState);
})

document.querySelector("#new").addEventListener("click", () => {
    window.open("/", "_blank").focus();
})

document.querySelector("#edit").addEventListener("click", async () => {
    const {key, content} = getState();
    let newState;
    let url;
    if (getUpdateToken(key) === "") {
        newState = {key: "", mode: "edit", content: content};
        url = "/";
    } else {
        newState = {key: key, mode: "edit", content: content};
        url = `/${key}`;
    }
    window.history.pushState(newState, "", url);

    updatePage(newState);
    updatePageButtons(newState);
})

document.addEventListener("keydown", async (event) => {
    if (event.ctrlKey  && event.key === 's') {
        if (!document.querySelector("#save").disabled)
            document.querySelector("#save").click();
    } else if (event.ctrlKey && event.key === 'x') {
        if (!document.querySelector("#new").disabled)
            document.querySelector("#new").click();
    } else if (event.ctrlKey && event.key === 'asd') {
        if (!document.querySelector("#copy").disabled)
            document.querySelector("#copy").click();
    } else if (event.ctrlKey && event.key === 'e') {
        if (!document.querySelector("#edit").disabled)
            document.querySelector("#edit").click();
    } else if (event.ctrlKey && event.shiftKey && event.key === 'd') {
        if (!document.querySelector("#delete").disabled) {
            let deleteConfirm = confirm("Are you sure you want to delete this document? This action cannot be undone.")
            if (!deleteConfirm) return;
            document.querySelector("#delete").click();
        }
    }
})


const showErrorPopup = () => {
    let popup = document.getElementById("error-popup");
    popup.classList.remove("hidden");
    setTimeout(() => popup.classList.add("hidden"), 5000);
}

document.querySelector("#save").addEventListener("click", async () => {
    const {key, mode, content} = getState()
    if (mode !== "edit") return;

    let saveButton = document.querySelector("#save");
    if (saveButton.style.disabled) return;
    const updateToken = getUpdateToken(key);

    saveButton.style.backgroundImage = 'url("/assets/icons/loading.gif")';
    saveButton.style.disabled = true;

    let response;
    if (key && updateToken) {
        response = await fetch(`/documents/${key}`, {
            method: "PATCH", body: content, headers: {
                Authorization: updateToken
            }
        });
    } else {
        response = await fetch("/documents", {
            method: "POST", body: content
        });
    }
    await new Promise(resolve => setTimeout(resolve, 4000));

    if (!response.ok) {
        showErrorPopup();
        saveButton.style.backgroundImage = 'url("/assets/icons/save.png")'
        saveButton.style.disabled = false;
        console.error("error from api: ", response);
        return;
    }

    const body = await response.json();
    setUpdateToken(body.key, body.update_token);

    const newState = {key: body.key, mode: "view", content: content};
    window.history.pushState(newState, "", `/${body.key}`);
    updatePage(newState);
    saveButton.style.backgroundImage = 'url("/assets/icons/save.png")'
    saveButton.style.disabled = false;

});

document.querySelector("#delete").addEventListener("click", async () => {
    const {key} = getState();
    const updateToken = getUpdateToken(key);
    if (updateToken === "") {
        console.error("no update token");
        return;
    }
    let deleteButton = document.querySelector("#delete");
    if (deleteButton.style.disabled) return;
    deleteButton.style.backgroundImage = 'url("/assets/icons/loading.gif")';
    deleteButton.style.disabled = true;

    let response = await fetch(`/documents/${key}`, {
        method: "DELETE", headers: {
            Authorization: updateToken
        }
    });
    if (!response.ok) {
        deleteButton.style.backgroundImage = 'url("/assets/icons/delete.png")'
        deleteButton.style.disabled = false;
        showErrorPopup()
        console.error("error from api: ", response);
        return;
    }
    deleteUpdateToken();
    const newState = {key: "", mode: "edit", content: ""};
    window.history.pushState(newState, "", "/");
    updatePage(newState);
    deleteButton.style.backgroundImage = 'url("/assets/icons/delete.png")'
    deleteButton.style.disabled = false;

})

document.querySelector("#copy").addEventListener("click", async () => {
    const data = window.history.state.content;
    if (!data) return;
    await navigator.clipboard.writeText(data);
})

document.querySelector("#raw").addEventListener("click", async () => {
    const {key} = getState();
    if (!key) return;
    window.open(`/raw/${key}`, "_blank").focus();
})

document.querySelector("#language").addEventListener("change", (event) => {
    highlightCode(event.target.value);
});

document.querySelector("#style").addEventListener("change", (event) => {
    setStyle(event.target.value);
});

function getState() {
    return window.history.state;
}

function getUpdateToken(key) {
    const documents = localStorage.getItem("documents")
    if (!documents) return ""
    const updateToken = JSON.parse(documents)[key]
    if (!updateToken) return ""

    return updateToken
}

function setUpdateToken(key, updateToken) {
    let documents = localStorage.getItem("documents")
    if (!documents) {
        documents = "{}"
    }
    const parsedDocuments = JSON.parse(documents)
    parsedDocuments[key] = updateToken
    localStorage.setItem("documents", JSON.stringify(parsedDocuments))
}

function deleteUpdateToken() {
    const {key} = getState();
    const documents = localStorage.getItem("documents");
    if (!documents) return;
    const parsedDocuments = JSON.parse(documents);
    delete parsedDocuments[key]
    localStorage.setItem("documents", JSON.stringify(parsedDocuments));
}

function updatePageButtons(state) {
    const {key, mode, content} = state;
    const updateToken = getUpdateToken(key);

    if (mode === "edit" && content && (!key || key && updateToken)) {
        document.querySelector("#save").disabled = false;
    } else {
        document.querySelector("#save").disabled = true;
    }

    if (mode === "edit" || !content) {
        document.querySelector("#new").disabled = true;
    } else {
        document.querySelector("#new").disabled = false;
    }

    if (updateToken) {
        document.querySelector("#delete").disabled = false;
        document.querySelector("#edit").disabled = false;
    } else {
        document.querySelector("#delete").disabled = true;
        document.querySelector("#edit").disabled = true;
    }

    if (key && mode === "view") {
        document.querySelector("#copy").disabled = false;
        document.querySelector("#raw").disabled = false;
    } else if (key) {
        document.querySelector("#copy").disabled = true;
        document.querySelector("#raw").disabled = true;
    }
}

function updatePage(state) {
    const {key, mode, content} = state;

    // update page title
    if (key) {
        document.title = `gobin - ${key}`;
    } else {
        document.title = "gobin";
    }

    if (mode === "view") {
        document.querySelector("#delete").disabled = false;
        document.querySelector("#save").disabled = true;
        document.querySelector("#edit").disabled = false;
        document.querySelector("#copy").disabled = false;
        document.querySelector("#raw").disabled = false;
        document.querySelector("#code-edit").style.display = "none";
        document.querySelector("#code-show").innerText = content;
        document.querySelector("#code").style.display = "block";
        highlightCode();
    } else if (mode === "edit") {
        document.querySelector("#delete").disabled = true;
        document.querySelector("#save").disabled = true;
        document.querySelector("#edit").disabled = true;
        document.querySelector("#copy").disabled = true;
        document.querySelector("#raw").disabled = true;
        document.querySelector("#code-show").innerText = "";
        document.querySelector("#code").style.display = "none";
        const codeEditElement = document.querySelector("#code-edit");
        codeEditElement.value = content;
        codeEditElement.style.display = "block";
    }
}

function setStyle(style) {
    localStorage.setItem("stylePreference", style)
    document.querySelector(`link[title="Highlight.js Style"]`).href = "https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.7.0/styles/" + style;
    document.querySelector("#style").value = style;
}

function highlightCode(language = undefined) {
    const codeElement = document.querySelector("#code-show");
    let result;
    if (language && language !== "auto") {
        result = hljs.highlight(codeElement.innerText, {
            language: language, ignoreIllegals: true
        });
    } else {
        result = hljs.highlightAuto(codeElement.innerText);
    }
    if (result.language === undefined) {
        result.language = "plaintext";
    }
    codeElement.innerHTML = result.value;
    codeElement.className = "hljs language-" + result.language;

    const languageElement = document.querySelector("#language");
    languageElement.value = result.language;

    if (result.value) {
        hljs.initLineNumbersOnLoad({singleLine: true});
    }
}