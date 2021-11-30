module.exports = {
    rules: {
        // Ensure we have a blank line between the header and the body.
       'body-leading-blank': [2, 'always'],
       // Warn on lines longer than 72 chars. We usually don't want them for
       // text, but links are fine if they exceed. So, just warn.
       'body-max-line-length': [1, 'always', 72],
       // Ensure the header line doesn't end with a period.
       'header-full-stop': [2, 'never'],
    },
};
