hljs.listLanguages().forEach((language) => {
    const option = document.createElement("option");
    option.value = language;
    option.innerText = language;
    document.querySelector("#language").appendChild(option);
});


document.addEventListener("DOMContentLoaded", () => {
    const style = localStorage.getItem("stylePreference") || "github-dark.min.css"
    setStyle(style);
    highlightCode();

    const key = getDocumentKey()
    if (!key || getUpdateToken(key) === "") {
        document.querySelector("#delete").disabled = true
    }

    if (!key) {
        document.querySelector("#edit").disabled = true
        document.querySelector("#raw").disabled = true
        document.querySelector("#copy").disabled = true
    }

    const codeEditElement = document.querySelector("#code-edit")
    codeEditElement.value = "";
    updateSaveButton(codeEditElement)
});

document.querySelector("#code-edit").addEventListener("keyup", (event) => {
    updateSaveButton(event.target)
})

document.querySelector("#new").addEventListener("click", () => {
    window.open("/", "_blank").focus();
})

document.querySelector("#edit").addEventListener("click", async () => {
    const data = await getDocumentRaw();
    if (!data) return;

    if (getUpdateToken(getDocumentKey()) === "") {
        window.history.pushState("Gobin", "", "/");
    }

    document.querySelector("#code").style.display = "none";
    const codeEditElement = document.querySelector("#code-edit");
    codeEditElement.value = data;
    codeEditElement.style.display = "block";
    document.querySelector("#delete").disabled = true;
    document.querySelector("#edit").disabled = true;
    document.querySelector("#copy").disabled = true;
    document.querySelector("#raw").disabled = true;
})

document.querySelector("#save").addEventListener("click", async () => {
    let data = document.querySelector("#code-edit").value;
    if (data.length === 0) return;

    const key = getDocumentKey();
    const updateToken = getUpdateToken(key);
    let response;
    if (key && updateToken) {
        response = await fetch(`/documents/${key}`, {
            method: "PATCH",
            body: data,
            headers: {
                Authorization: updateToken
            }
        });
    } else {
        response = await fetch("/documents", {
            method: "POST",
            body: data
        });
    }

    if (!response.ok) {
        console.error("error from api: ", response);
        return;
    }

    const body = await response.json();
    setUpdateToken(body.key, body.update_token);

    window.history.pushState(`Gobin - ${body.key}`, "", `/${body.key}`);
    document.querySelector("#delete").disabled = false;
    document.querySelector("#save").disabled = true;
    document.querySelector("#edit").disabled = false;
    document.querySelector("#copy").disabled = false;
    document.querySelector("#raw").disabled = false;
    document.querySelector("#code-edit").style.display = "none";
    document.querySelector("#code-show").innerText = data;
    document.querySelector("#code").style.display = "block";
    highlightCode();
});

document.querySelector("#delete").addEventListener("click", async () => {
    const key = getDocumentKey();
    const updateToken = getUpdateToken(key);
    if (updateToken === "") {
        console.error("no update token");
        return;
    }

    let response = await fetch(`/documents/${key}`, {
        method: "DELETE",
        headers: {
            Authorization: updateToken
        }
    });
    if (!response.ok) {
        console.error("error from api: ", response);
        return;
    }
    deleteUpdateToken();
    document.querySelector("#delete").disabled = true;
    document.querySelector("#save").disabled = true;
    document.querySelector("#edit").disabled = true;
    document.querySelector("#copy").disabled = true;
    document.querySelector("#raw").disabled = true;
    document.querySelector("#code-show").innerText = "";
    document.querySelector("#code").style.display = "none";
    const codeEditElement = document.querySelector("#code-edit");
    codeEditElement.value = "";
    codeEditElement.style.display = "block";

    window.history.pushState("Gobin", "", "/");
})

document.querySelector("#copy").addEventListener("click", async () => {
    const data = await getDocumentRaw();
    if (!data) return;
    await navigator.clipboard.writeText(data);
})

document.querySelector("#raw").addEventListener("click", async () => {
    window.open(`/raw/${getDocumentKey()}`, "_blank").focus();
})

document.querySelector("#language").addEventListener("change", (event) => {
    highlightCode(event.target.value);
});

document.querySelector("#style").addEventListener("change", (event) => {
    setStyle(event.target.value);
});

async function getDocumentRaw() {
    const key = getDocumentKey()
    const response = await fetch(`/raw/${key}`, {
        method: "GET"
    });

    if (!response.ok) {
        console.error("error from api: ", response);
        return "";
    }
    return await response.text();
}

function updateSaveButton(element) {
    if (element.value.length === 0) {
        document.querySelector("#save").disabled = true;
        return
    }
    document.querySelector("#save").disabled = false;
}

function getDocumentKey() {
    const paths = window.location.pathname.split("/")
    if (paths.length < 2) return ""
    return paths[1]
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
    const key = getDocumentKey()
    const documents = localStorage.getItem("documents")
    if (!documents) return
    const parsedDocuments = JSON.parse(documents)
    delete parsedDocuments[key]
    localStorage.setItem("documents", JSON.stringify(parsedDocuments))
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
            language: language,
            ignoreIllegals: true
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